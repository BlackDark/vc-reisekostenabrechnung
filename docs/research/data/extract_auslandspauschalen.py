import re,csv,sys
HDR=re.compile(r'Pauschbetr|Übersicht|Verpflegungsmehr|aufwendungen|Abwesen|heitsdauer|Kalendertag|mindestens|Abreisetag|Übernach|Land\b|^\s*€|^\s*\d+\s*$|Änderungen|Stunden|tungskosten|einer|von mehr|je\s*$|sowie bei|für den An|und\s*$|Seite')
def parse(fn):
    rows=[];parent=None;last=None  # last: 'sub','country','parentname',None
    for raw in open(fn,encoding='utf-8'):
        l=raw.rstrip()
        if not l.strip(): continue
        m=re.match(r'^\s*([–-]\s+)?(\S.*?)\s{2,}(\d+)\s+(\d+)\s+(\d+)\s*$',l)
        if m:
            sub,name,a,b,c=m.groups()
            if sub: rows.append([parent,name.strip(),int(a),int(b),int(c)]); last='sub'
            else: parent=None; rows.append([name.strip(),'',int(a),int(b),int(c)]); last='country'
            continue
        if HDR.search(l): continue
        txt=l.strip()
        if last=="sub" and re.search(r"(die|sowie|und|,)$", rows[-1][1]):
            rows[-1][1]+=' '+txt; continue
        if last=='parentname':
            parent+=' '+txt; continue
        parent=txt; last='parentname'
    return rows
for y in sys.argv[1:]:
    r=parse(f'aus{y}.txt')
    with open(f'auslandspauschalen_{y}.csv','w',newline='',encoding='utf-8') as f:
        w=csv.writer(f);w.writerow(['jahr','land','ort','vma_24h_eur','vma_an_abreise_8h_eur','uebernachtung_ag_pauschal_eur'])
        for x in r: w.writerow([y]+x)
    print(y,len(r))
