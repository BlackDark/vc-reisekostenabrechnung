package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/belegpipe"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/mailauth"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
)

const maxMailAttachments = 20

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// classifyOne sniffs and validates a single file and returns its upload kind,
// mirroring classifyUploads for the multi-file browser path but for one file.
func classifyOne(b []byte) (uploadKind, error) {
	if len(b) == 0 {
		return uploadKind{}, errors.New("empty file")
	}
	kind := belegpipe.Sniff(b)
	if kind == "" {
		return uploadKind{}, errors.New("unknown type")
	}
	switch kind {
	case "jpeg", "png", "webp":
		if _, _, err := belegpipe.DecodeLimited(bytes.NewReader(b), belegpipe.MaxPixels); err != nil {
			return uploadKind{}, err
		}
		return uploadKind{typ: "foto", status: "in_aufbereitung", job: "aufbereiten", pages: 1}, nil
	case "pdf":
		pages, err := belegpipe.PDFPages(b)
		if err != nil {
			return uploadKind{}, err
		}
		typ := "pdf"
		if belegpipe.PDFHybrid(b) {
			typ = "e_rechnung_hybrid"
		}
		return uploadKind{typ: typ, status: "in_aufbereitung", job: "pdf_vorschau", pages: pages}, nil
	case "xml":
		if _, err := belegpipe.XMLRoot(b); err != nil {
			return uploadKind{}, err
		}
		return uploadKind{typ: "e_rechnung_xml", status: "zur_bestaetigung", pages: 1}, nil
	default:
		return uploadKind{}, errors.New("unknown type")
	}
}

// extractMailAttachments returns the bytes of the explicit attachments of an
// email, skipping inline bodies and inline images (logos, signatures). Content
// type is judged by magic bytes at classification, never the file name.
func (a *App) extractMailAttachments(msg *mail.Message) [][]byte {
	mt, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mt, "multipart/") {
		return nil
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	out := make([][]byte, 0, 4)
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		disp, _, _ := mime.ParseMediaType(p.Header.Get("Content-Disposition"))
		if disp == "inline" || (disp != "attachment" && p.FileName() == "") {
			_ = p.Close()
			continue
		}
		pct, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if pct == "text/html" || pct == "text/plain" || strings.HasPrefix(pct, "multipart/") {
			_ = p.Close()
			continue
		}
		b, rerr := io.ReadAll(io.LimitReader(p, a.maxUpload()))
		_ = p.Close()
		if rerr != nil || len(b) == 0 || belegpipe.Sniff(b) == "" {
			continue
		}
		out = append(out, b)
		if len(out) >= maxMailAttachments {
			break
		}
	}
	return out
}

// PostEingangMail parses a raw email and files each attachment as a Beleg in the
// caller's Belegeingang; the original .eml is archived for provenance (GoBD) and
// each duplicate attachment is skipped by checksum. This is the consume half of
// email intake (design note options a/c). Sender verification and the Nutzer
// lookup happen in ingestMailFor, and the IMAP poller feeds this endpoint.
func (a *App) PostEingangMail(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireTokenActor(w, r, store.TokenScopeMailIngest)
	if !ok {
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, a.maxUpload()))
	if err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			writeProblem(w, http.StatusRequestEntityTooLarge, "datei_zu_gross", "File is too large", "")
			return
		}
		writeProblem(w, http.StatusBadRequest, "ungueltige_anfrage", "Bad request", "")
		return
	}
	items, ierr := a.ingestMailFor(r.Context(), n.ID, raw, a.actor(r, n))
	if ierr != nil {
		if errors.Is(ierr, errMailInvalid) {
			writeProblem(w, http.StatusUnprocessableEntity, "ungueltige_mail", "Not a valid email", "")
			return
		}
		if errors.Is(ierr, errMailAuth) {
			writeProblem(w, http.StatusUnprocessableEntity, "mail_auth", "Mail sender could not be verified", "")
			return
		}
		writeStoreErr(w, ierr)
		return
	}
	writeJSON(w, http.StatusAccepted, api.BelegListe{Items: items})
}

// errMailInvalid marks an empty or unparsable email (a client error, 422).
var errMailInvalid = errors.New("not a valid email")

// errMailAuth marks an email whose sender could not be proven. The mail is
// dropped rather than retried: no amount of re-fetching changes what the
// receiving server already stamped into the header.
var errMailAuth = errors.New("mail sender authentication failed")

// ingestMailFor parses a raw email and files each attachment as a Beleg in the
// given owner's Belegeingang, archiving the original .eml for GoBD and skipping
// duplicate attachments by checksum. It is shared by the HTTP ingest endpoint and
// the mail poller (design note options a/c); ctx is the request or job context.
//
// An email whose Authentication-Results verdict names an unproven sender is
// refused with errMailAuth. The verdict is read from the header the mailbox's
// own server wrote, never re-derived here, so a mail that arrives without the
// header is filed as unverified rather than rejected.
func (a *App) ingestMailFor(ctx context.Context, nutzerID string, raw []byte, actor store.Actor) ([]api.Beleg, error) {
	if len(raw) == 0 {
		return nil, errMailInvalid
	}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, errMailInvalid
	}
	auth := mailauth.Parse(msg)
	if auth.Rejected() {
		return nil, fmt.Errorf("%w: %s", errMailAuth, auth.Summary())
	}
	sha := belegpipe.SHA256Hex(raw)
	emlKey := ""
	if err := a.blobs.PutIfAbsent(ctx, fmt.Sprintf("mail/%s/%s.eml", nutzerID, sha), bytes.NewReader(raw), sha); err != nil {
		a.log.Error("archive mail", "err", err)
	} else {
		emlKey = fmt.Sprintf("mail/%s/%s.eml", nutzerID, sha)
	}
	origin := mailOrigin(msg)
	items := make([]api.Beleg, 0)
	for _, b := range a.extractMailAttachments(msg) {
		kind, cerr := classifyOne(b)
		if cerr != nil {
			continue
		}
		one := [][]byte{b}
		original, parts := hashParts(one)
		_, found, derr := a.store.FindBelegDuplicate(ctx, nutzerID, original, parts)
		if derr != nil {
			return items, derr
		}
		if found {
			continue
		}
		belegID := id.Must()
		stored, dateien, serr := a.storeOriginals(ctx, nutzerID, belegID, kind, one)
		if serr != nil {
			a.dropKeys(ctx, stored, "mail.eingegangen")
			return items, serr
		}
		row, cerr := a.store.CreateBeleg(ctx, nutzerID, store.NewBeleg{
			ID: belegID, Typ: kind.typ, Seiten: int64(kind.pages), SHA256: original,
			Status: kind.status, Dateien: dateien, JobArt: kind.job,
			Quelle: "mail",
			Mail: &store.BelegMailInput{
				Absender: origin.absender, Betreff: origin.betreff,
				MailDatum: origin.datum, EmlSchluessel: emlKey,
				MailAuth: auth.Summary(),
			},
		}, actor)
		if cerr != nil {
			a.dropKeys(ctx, stored, "mail.eingegangen")
			return items, cerr
		}
		items = append(items, toBeleg(row))
	}
	return items, nil
}

// mailInfo is the header summary a mail-filed Beleg records as its origin.
type mailInfo struct {
	absender string
	betreff  string
	datum    string
}

// mailOrigin reads the fields worth keeping about an email. A missing sender or
// subject is left empty rather than dropped, because the attachments still belong
// to the recipient's inbox.
func mailOrigin(msg *mail.Message) mailInfo {
	from := strings.TrimSpace(msg.Header.Get("From"))
	if addr, err := mail.ParseAddress(from); err == nil {
		from = addr.Address
	}
	datum := strings.TrimSpace(msg.Header.Get("Date"))
	if parsed, err := msg.Header.Date(); err == nil {
		datum = parsed.UTC().Format(time.RFC3339)
	}
	return mailInfo{
		absender: from,
		betreff:  strings.TrimSpace(msg.Header.Get("Subject")),
		datum:    datum,
	}
}
