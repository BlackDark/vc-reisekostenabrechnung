import sys,os,json
sys.argv=['x']
src=open('exp.py').read().split("if __name__")[0]; exec(src)
syn=open('synth.py').read().split('# 1) A4')[0]; exec(syn)
from PIL import ImageDraw
os.makedirs('fade',exist_ok=True)
lines=['   ARAL Tankstelle','  Inh. Jürgen Weiß e.K.',' Münchner Str. 45',' 85540 Haar','StNr. 143/221/90876','Säule 4  Super E10','  42,31 l x 1,789 EUR/l','                   75,69 EUR A','SUMME EUR           93,79','MwSt   Netto   MwSt  Brutto','A 19%  78,82  14,97  93,79','Datum: 22.09.2026  Zeit: 07:42','Beleg-Nr. 0048213  Kasse 2']
keys=KEYS['17_tank_thermo_faded_synth']
out=[]
for ink in (150,180,200,212,220,226):
    im=Image.new('RGB',(900,1300),(238,236,228)); d=ImageDraw.Draw(im); y=60
    for t in lines: d.text((40,y),t,font=font('DejaVuSansMono.ttf',34),fill=(ink,ink,ink-4)); y+=53
    p=f'fade/ink{ink}.jpg'; photo(im,p,size=(2250,3000))
    a=load(p); w,_=dewarp(a); w=to_300dpi(w,'bon'); g=bg_normalize(cv2.cvtColor(w,cv2.COLOR_RGB2GRAY))
    Image.fromarray(g).save(f'fade/ink{ink}_gray.png'); sh(f'cwebp -quiet -q 50 fade/ink{ink}_gray.png -o fade/ink{ink}_gray.webp'); Image.open(f'fade/ink{ink}_gray.webp').save(f'fade/ink{ink}_gray_dec.png')
    bw=sauvola(g); Image.fromarray(bw).convert('1').save(f'fade/ink{ink}_bw.png')
    r={'ink':ink,'contrast':238-ink}
    for v,f in (('gray_webp50',f'fade/ink{ink}_gray_dec.png'),('bilevel',f'fade/ink{ink}_bw.png'),('original',p)):
        sh(f'OMP_THREAD_LIMIT=1 tesseract {f} fade/o_{ink}_{v} -l deu+eng --psm 4'); t=open(f'fade/o_{ink}_{v}.txt').read()
        r[v]=keyscore(t,keys)[0]
    r['webp_kb']=os.path.getsize(f'fade/ink{ink}_gray.webp')//1024
    print(r,flush=True); out.append(r)
json.dump(out,open('fade/res.json','w'))
