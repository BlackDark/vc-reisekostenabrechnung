# Synthetische, "abfotografierte" Belege mit exakter Ground Truth
import numpy as np, cv2
from PIL import Image, ImageDraw, ImageFont
rng=np.random.default_rng(42)
F='/usr/share/fonts/truetype/dejavu/'
def font(n,s): return ImageFont.truetype(F+n,s)

def photo(img, out, size=(3000,4000), shadow=True, q=92):
    a=np.array(img.convert('RGB')).astype(np.float32)
    h,w=a.shape[:2]; W,H=size
    # paper on table: place with perspective
    bg=np.zeros((H,W,3),np.float32); bg[:]=(92,74,58)  # wooden table
    bg+=rng.normal(0,6,(H,W,1))
    src=np.float32([[0,0],[w,0],[w,h],[0,h]])
    s=min(W*0.86/w,H*0.88/h)
    cx,cy=W/2,H/2; hw,hh=w*s/2,h*s/2
    dst=np.float32([[cx-hw+60,cy-hh+40],[cx+hw-20,cy-hh+110],[cx+hw+30,cy+hh-30],[cx-hw-40,cy+hh+10]])
    M=cv2.getPerspectiveTransform(src,dst)
    warped=cv2.warpPerspective(a,M,(W,H),flags=cv2.INTER_AREA,borderValue=(0,0,0))
    mask=cv2.warpPerspective(np.ones((h,w),np.float32),M,(W,H))
    mask=cv2.GaussianBlur(mask,(5,5),0)[...,None]
    comp=warped*mask+bg*(1-mask)
    # lighting gradient + soft phone shadow
    yy,xx=np.mgrid[0:H,0:W].astype(np.float32)
    light=0.80+0.22*(xx/W)+0.05*(yy/H)
    if shadow:
        sh=np.clip(((xx-0.15*W)*0.6+(yy-0.55*H))/(0.08*W),0,1)  # diagonal shadow edge
        light*=1-0.32*(1-sh)*(yy>0.45*H)
        light=cv2.GaussianBlur(light,(0,0),25)
    comp*=light[...,None]
    comp*=np.array([1.0,0.97,0.90])  # warm white balance
    comp=cv2.GaussianBlur(comp,(0,0),0.9)  # slight defocus
    comp+=rng.normal(0,3.5,comp.shape)
    comp=np.clip(comp,0,255).astype(np.uint8)
    Image.fromarray(comp).save(out,quality=q,subsampling=2)

# 1) A4 Hotelrechnung (300 dpi)
W,H=2480,3508
im=Image.new('RGB',(W,H),(250,250,247)); d=ImageDraw.Draw(im)
L=[]
def t(x,y,s,txt,f='DejaVuSans.ttf',fill=(25,25,25)):
    d.text((x,y),txt,font=font(f,s),fill=fill); L.append(txt)
t(180,180,72,'Hotel Lindenhof Leipzig','DejaVuSerif-Bold.ttf',(20,40,90))
t(180,280,34,'Lindenhof Hotelbetriebs GmbH · Karl-Liebknecht-Str. 12 · 04107 Leipzig')
t(180,330,34,'USt-IdNr.: DE298765431 · Steuernr.: 231/108/04512')
t(180,520,40,'VC Beispiel GmbH')
t(180,575,40,'z. Hd. Herrn Eduard Marbach')
t(180,630,40,'Musterweg 7')
t(180,685,40,'80331 München')
t(1500,520,36,'Rechnungsnr.: 2026-104733')
t(1500,570,36,'Rechnungsdatum: 17.09.2026')
t(1500,620,36,'Zimmer: 412  Gast: E. Marbach')
t(1500,670,36,'Anreise: 15.09.2026 Abreise: 17.09.2026')
t(180,860,60,'Rechnung','DejaVuSans-Bold.ttf')
y=980
t(180,y,36,'Datum       Leistung                         Menge   Einzel     USt   Betrag','DejaVuSansMono.ttf'); y+=70
rows=[('15.09.2026','Übernachtung Einzelzimmer','1','119,00 EUR',' 7%','119,00 EUR'),
('15.09.2026','Frühstück','1',' 18,50 EUR','19%',' 18,50 EUR'),
('16.09.2026','Übernachtung Einzelzimmer','1','119,00 EUR',' 7%','119,00 EUR'),
('16.09.2026','Frühstück','1',' 18,50 EUR','19%',' 18,50 EUR'),
('16.09.2026','Parken Tiefgarage','2',' 12,00 EUR','19%',' 24,00 EUR'),
('16.09.2026','Minibar Wasser 0,5 l','2','  3,50 EUR','19%','  7,00 EUR')]
for r in rows:
    t(180,y,36,f'{r[0]}  {r[1]:<32} {r[2]:>3}  {r[3]}  {r[4]}  {r[5]}','DejaVuSansMono.ttf'); y+=60
d.line((180,y+10,2300,y+10),fill=(40,40,40),width=3); y+=40
t(1200,y,38,'Summe netto           268,00 EUR','DejaVuSansMono.ttf'); y+=60
t(1200,y,38,'zzgl. 7% USt auf 238,00  16,66 EUR','DejaVuSansMono.ttf'); y+=60
t(1200,y,38,'zzgl. 19% USt auf 68,00  12,92 EUR','DejaVuSansMono.ttf'); y+=60
t(1200,y,44,'Gesamtbetrag         297,58 EUR','DejaVuSansMono-Bold.ttf'); y+=120
t(180,y,36,'Bezahlt per Kreditkarte VISA ****4821 am 17.09.2026.'); y+=60
t(180,y,36,'Vielen Dank für Ihren Aufenthalt!'); y+=600
for s in ['Geschäftsführer: Petra Schönfeld · Amtsgericht Leipzig HRB 33871',
          'Bankverbindung: Sparkasse Leipzig · IBAN DE12 8605 5592 1100 4455 66 · BIC WELADE8LXXX',
          'Hinweis: Die Leistungen wurden im Leistungszeitraum 15.09.2026 bis 17.09.2026 erbracht.']:
    t(180,y,26,s,fill=(120,120,120)); y+=45
# blue stamp
st=Image.new('RGBA',(700,260),(0,0,0,0)); sd=ImageDraw.Draw(st)
sd.rounded_rectangle((8,8,692,252),30,outline=(30,60,200,170),width=10)
sd.text((60,40),'BEZAHLT',font=font('DejaVuSans-Bold.ttf',110),fill=(30,60,200,170))
sd.text((150,185),'17.09.2026',font=font('DejaVuSans-Bold.ttf',50),fill=(30,60,200,170))
st=st.rotate(12,expand=True,resample=Image.BICUBIC)
im.paste(st,(1350,y-1050),st)
open('gt/16_hotel_a4_synth.txt','w').write('\n'.join(L))
photo(im,'src/16_hotel_a4_synth.jpg')

# 2) Verblasster Thermobon (Tankquittung)
W,H=900,2200
im=Image.new('RGB',(W,H),(238,236,228)); d=ImageDraw.Draw(im); L=[]
ink=(150,150,150)
def m(y,txt,s=34,b=False):
    g=int(135+60*min(1,y/2000)); d.text((40,y),txt,font=font('DejaVuSansMono-Bold.ttf' if b else 'DejaVuSansMono.ttf',s),fill=(g,g,g-4)); L.append(txt)
lines=[('   ARAL Tankstelle',44,True),('  Inh. Jürgen Weiß e.K.',34,False),(' Münchner Str. 45',34,False),(' 85540 Haar',34,False),('Tel. 089 4612345',34,False),('',34,False),
('StNr. 143/221/90876',34,False),('',34,False),('Säule 4  Super E10',34,False),('  42,31 l x 1,789 EUR/l',34,False),('                   75,69 EUR A',34,False),
('Autowäsche Premium  14,90 EUR A',34,False),('Kaffee to go         3,20 EUR A',34,False),('',34,False),('SUMME EUR           93,79',40,True),('',34,False),
('Girocard            93,79 EUR',34,False),('',34,False),('MwSt   Netto   MwSt  Brutto',34,False),('A 19%  78,82  14,97  93,79',34,False),('',34,False),
('Datum: 22.09.2026  Zeit: 07:42',34,False),('Beleg-Nr. 0048213  Kasse 2',34,False),('TSE-Signatur-Zähler: 772104',34,False),('TSE-Start: 2026-09-22T07:41:58',34,False),
('',34,False),('  Gute Fahrt!',34,False)]
y=60
for txt,s,b in lines: m(y,txt,s,b); y+=int(s*1.55)
L=[l for l in L if l.strip()]
open('gt/17_tank_thermo_faded_synth.txt','w').write('\n'.join(L))
im=im.crop((0,0,W,y+80))
photo(im,'src/17_tank_thermo_faded_synth.jpg',size=(3000,4000))
