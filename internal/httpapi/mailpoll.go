package httpapi

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/mailpoll"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
)

// StartMailPoller runs the email-receipt intake loop (design note a) until ctx is
// cancelled. It is a no-op unless MAIL_IMAP_HOST is configured. Each tick it polls
// the shared mailbox for unseen mail, files each attachment into the recipient
// owner's Belegeingang, and marks the message processed. The IMAP transport is
// the one part not covered by a durable unit test here; it is meant to be verified
// against a containerized IMAP server (Dovecot/GreenMail) as the design note says.
func (a *App) StartMailPoller(ctx context.Context) {
	cfg := a.cfg.Mail
	if cfg.IMAPHost == "" {
		return
	}
	if cfg.IMAPInterval <= 0 {
		a.log.Error("mail poller disabled: MAIL_IMAP_INTERVAL must be positive")
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if n, err := a.pollMailOnce(ctx); err != nil {
			a.log.Error("mail poll failed", "err", err)
		} else if n > 0 {
			a.log.Info("mail intake filed receipts", "count", n)
		}
		timer := time.NewTimer(cfg.IMAPInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (a *App) pollMailOnce(ctx context.Context) (int, error) {
	cfg := a.cfg.Mail
	port := mailpoll.NormalizePort(cfg.IMAPPort)
	if port == 0 {
		return 0, fmt.Errorf("invalid MAIL_IMAP_PORT %q", cfg.IMAPPort)
	}
	client, err := imapclient.DialTLS(net.JoinHostPort(cfg.IMAPHost, strconv.Itoa(port)), nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = client.Logout().Wait() }()

	if err := client.Login(cfg.IMAPUser, cfg.IMAPPassword).Wait(); err != nil {
		return 0, err
	}
	if _, err := client.Select(cfg.IMAPFolder, nil).Wait(); err != nil {
		return 0, err
	}
	found, err := client.Search(&imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}, nil).Wait()
	if err != nil {
		return 0, err
	}
	if found.Count == 0 {
		return 0, nil
	}
	fetch := client.Fetch(found.All, &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{{}},
		Envelope:    true,
	})
	handled := 0
	var processed imap.SeqSet
	for msg := fetch.Next(); msg != nil; msg = fetch.Next() {
		var body []byte
		var env *imap.Envelope
		for item := msg.Next(); item != nil; item = msg.Next() {
			switch v := item.(type) {
			case *imapclient.FetchItemDataBodySection:
				if v.Literal != nil {
					body, _ = io.ReadAll(v.Literal)
				}
			case *imapclient.FetchItemDataEnvelope:
				env = v.Envelope
			}
		}
		nutzerID, ok := a.resolveMailUser(ctx, env)
		if !ok || len(body) == 0 {
			continue
		}
		if _, ierr := a.ingestMailFor(ctx, nutzerID, body, store.Actor{Art: "mail"}); ierr != nil {
			a.log.Error("mail ingest failed; leaving the message unseen for retry", "err", ierr)
			continue
		}
		seq := uint32(msg.SeqNum)
		processed = append(processed, imap.SeqRange{Start: seq, Stop: seq})
		handled++
	}
	if err := fetch.Close(); err != nil {
		return handled, err
	}
	if len(processed) > 0 {
		a.markProcessed(client, processed)
	}
	return handled, nil
}

// resolveMailUser maps a fetched message to its recipient Nutzer: first a
// plus-address belege+<mail:ingest token>, then the sender allowlist.
func (a *App) resolveMailUser(ctx context.Context, env *imap.Envelope) (string, bool) {
	if env == nil {
		return "", false
	}
	for _, to := range env.To {
		if !strings.HasPrefix(to.Mailbox, "belege+") {
			continue
		}
		raw := strings.TrimPrefix(to.Mailbox, "belege+")
		if raw == "" {
			continue
		}
		tok, err := a.store.FindActiveToken(ctx, auth.HashAPIToken(raw))
		if err == nil && tok.Scope == store.TokenScopeMailIngest {
			return tok.NutzerID, true
		}
	}
	if len(env.From) > 0 {
		from := env.From[0]
		id, ok, err := a.store.MailOwnerForAbsender(ctx, from.Mailbox+"@"+from.Host)
		if err == nil && ok {
			return id, true
		}
	}
	return "", false
}

func (a *App) markProcessed(client *imapclient.Client, set imap.SeqSet) {
	cfg := a.cfg.Mail
	switch cfg.ProcessedAction {
	case "flag":
		_ = client.Store(set, &imap.StoreFlags{
			Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagSeen},
		}, nil).Close()
	case "delete":
		_ = client.Store(set, &imap.StoreFlags{
			Op: imap.StoreFlagsAdd, Flags: []imap.Flag{imap.FlagDeleted},
		}, nil).Close()
		_ = client.Expunge().Close()
	default:
		if mailbox, ok := mailpoll.ProcessedMailbox(cfg.ProcessedAction); ok {
			_ = client.Create(mailbox, nil).Wait()
			_, _ = client.Move(set, mailbox).Wait()
		}
	}
}
