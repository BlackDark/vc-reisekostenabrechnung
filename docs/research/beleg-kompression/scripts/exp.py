import os, sys, subprocess, json, glob, io, time
import numpy as np, cv2
from PIL import Image
import pillow_avif
from skimage.filters import threshold_sauvola
from rapidfuzz.distance import Levenshtein
import pikepdf
from keys import KEYS
DPI=int(os.environ.get('DPI','300')); QUICK=os.environ.get('QUICK')=='1'
B='/workspace/beleg-exp'; OUT=B+('/out' if DPI==300 else f'/out_dpi{DPI}')
# Simuliert manuelles Nachziehen der Ecken im UI, wo die Auto-Erkennung versagt
MANUAL_QUAD={'03_aldi_hesel':[[665,537],[1957,489],[2100,4332],[736,4346]]}
DOC_TYPE={'05_supermarkt_pl':'bon'}; os.makedirs(OUT,exist_ok=True)
def sh(cmd): subprocess.run(cmd,shell=True,check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)

def load(path):
    im=Image.open(path); 
    from PIL import ImageOps; im=ImageOps.exif_transpose(im)
    return np.array(im.convert('RGB'))

def resize_long(a,L):
    h,w=a.shape[:2]; s=L/max(h,w)
    if s>=1: return a
    return cv2.resize(a,(round(w*s),round(h*s)),interpolation=cv2.INTER_AREA)

def find_quad(a):
    h,w=a.shape[:2]; s=1000/max(h,w); sm=cv2.resize(a,(int(w*s),int(h*s)),interpolation=cv2.INTER_AREA)
    sm=cv2.GaussianBlur(sm,(5,5),0)
    # paper = bright & low saturation region
    hsv=cv2.cvtColor(sm,cv2.COLOR_RGB2HSV)
    sat=hsv[...,1].astype(np.float32); val=hsv[...,2].astype(np.float32)
    score=np.clip(val-1.5*sat,0,255).astype(np.uint8)
    _,th=cv2.threshold(score,0,255,cv2.THRESH_BINARY+cv2.THRESH_OTSU)
    th=cv2.morphologyEx(th,cv2.MORPH_CLOSE,np.ones((15,15),np.uint8))
    cs,_=cv2.findContours(th,cv2.RETR_EXTERNAL,cv2.CHAIN_APPROX_SIMPLE)
    if not cs: return None
    c=max(cs,key=cv2.contourArea); area=cv2.contourArea(c)
    if area<0.10*sm.shape[0]*sm.shape[1] or area>0.92*sm.shape[0]*sm.shape[1]: return None  # scan / no clear paper edge
    hull=cv2.convexHull(c); peri=cv2.arcLength(hull,True)
    ap=cv2.approxPolyDP(hull,0.02*peri,True)
    if len(ap)==4: q=ap.reshape(4,2).astype(np.float32)
    else: q=cv2.boxPoints(cv2.minAreaRect(hull)).astype(np.float32)
    return q/s

def order(q):
    c=q.mean(0); ang=np.arctan2(q[:,1]-c[1],q[:,0]-c[0]); q=q[np.argsort(ang)]  # clockwise in image coords
    # start with the corner that is top-left relative to the quad's own long axis
    i=int(np.argmin(q.sum(1))); q=np.roll(q,-i,axis=0)
    e01=np.linalg.norm(q[1]-q[0]); e12=np.linalg.norm(q[2]-q[1])
    # q0->q1 should be the top edge (more horizontal than vertical)
    v=q[1]-q[0]
    if abs(v[1])>abs(v[0]): q=np.roll(q,1,axis=0)
    return np.float32(q)

def dewarp(a,name=None):
    q=np.float32(MANUAL_QUAD[name]) if name in MANUAL_QUAD else find_quad(a)
    if q is None: return a, False
    q=order(q); (tl,tr,br,bl)=q
    W=int(max(np.linalg.norm(tr-tl),np.linalg.norm(br-bl))); H=int(max(np.linalg.norm(bl-tl),np.linalg.norm(br-tr)))
    M=cv2.getPerspectiveTransform(q,np.float32([[0,0],[W,0],[W,H],[0,H]]))
    return cv2.warpPerspective(a,M,(W,H),flags=cv2.INTER_CUBIC,borderMode=cv2.BORDER_REPLICATE), True

def trim(a):
    g=cv2.cvtColor(a,cv2.COLOR_RGB2GRAY); s=800/max(g.shape); sm=cv2.resize(g,None,fx=s,fy=s,interpolation=cv2.INTER_AREA)
    n=bg_normalize(sm); ink=(n<110).astype(np.uint8)
    ink=cv2.morphologyEx(ink,cv2.MORPH_OPEN,np.ones((2,2),np.uint8))
    ys,xs=np.nonzero(ink)
    if len(xs)<50: return a
    x0,x1=np.percentile(xs,[0.2,99.8]); y0,y1=np.percentile(ys,[0.2,99.8]); m=0.03*max(sm.shape)
    x0,y0=max(0,int((x0-m)/s)),max(0,int((y0-m)/s)); x1,y1=int((x1+m)/s),int((y1+m)/s)
    return a[y0:y1,x0:x1]

def to_300dpi(a,kind=None):
    h,w=a.shape[:2]; ar=max(h,w)/min(h,w)
    # Annahme physische Breite: Kassenbon 80 mm, sonst A4 210 mm (Hochformat)
    mm=80 if (kind=='bon' or (kind is None and ar>1.6)) else 210
    tw=round(mm/25.4*DPI)
    if w<=tw: return a
    s=tw/w; return cv2.resize(a,(tw,round(h*s)),interpolation=cv2.INTER_AREA)

def bg_normalize(g):
    # g: uint8 single channel. Divide by estimated background (paper) -> removes shadows/gradients
    k=max(15,(min(g.shape)//25)|1)
    bg=cv2.morphologyEx(g,cv2.MORPH_CLOSE,cv2.getStructuringElement(cv2.MORPH_ELLIPSE,(k,k)))
    bg=cv2.GaussianBlur(bg,(0,0),k/3)
    n=np.clip(g.astype(np.float32)/np.maximum(bg,1)*255,0,255)
    lo=np.percentile(n,0.5); hi=min(250.0,np.percentile(n,60))  # paper -> white
    n=np.clip((n-lo)/max(hi-lo,1)*255,0,255)
    return n.astype(np.uint8)

def clean_color(a):
    return np.dstack([bg_normalize(a[...,i]) for i in range(3)])

def sauvola(g):
    w=max(25,(g.shape[1]//30)|1)
    t=threshold_sauvola(g,window_size=w,k=0.2)
    return (g>t).astype(np.uint8)*255

def save_bytes(fn): return os.path.getsize(fn)

def ocr(img_path,name):
    o=f'{OUT}/ocr/{name}'; os.makedirs(os.path.dirname(o),exist_ok=True)
    env='OMP_THREAD_LIMIT=1'
    sh(f'{env} tesseract "{img_path}" "{o}" -l deu+eng --psm 4')
    return open(o+'.txt').read()

def keyscore(txt,keys):
    t=''.join(txt.split())
    hit=[k for k in keys if ''.join(k.split()) in t]
    return len(hit), hit

def cer(txt,gt):
    a=' '.join(txt.split()); b=' '.join(gt.split())
    return Levenshtein.distance(a,b)/len(b)

def pdf_jbig2(png,pdf,w,h,dpi):
    jb=png[:-4]+'.jb2'
    sh(f'jbig2 -p "{png}" > "{jb}"')
    data=open(jb,'rb').read()
    p=pikepdf.new()
    img=pikepdf.Stream(p,data); img.Type=pikepdf.Name.XObject; img.Subtype=pikepdf.Name.Image
    img.Width=w; img.Height=h; img.ColorSpace=pikepdf.Name.DeviceGray; img.BitsPerComponent=1; img.Filter=pikepdf.Name.JBIG2Decode
    pw,ph=w*72/dpi,h*72/dpi
    content=pikepdf.Stream(p,f'q {pw:.2f} 0 0 {ph:.2f} 0 0 cm /Im0 Do Q'.encode())
    page=pikepdf.Dictionary(Type=pikepdf.Name.Page,MediaBox=[0,0,pw,ph],Contents=content,Resources=pikepdf.Dictionary(XObject=pikepdf.Dictionary(Im0=img)))
    p.pages.append(pikepdf.Page(page)); p.save(pdf,compress_streams=True,object_stream_mode=pikepdf.ObjectStreamMode.generate)
    os.remove(jb)

def run(path):
    name=os.path.splitext(os.path.basename(path))[0]; d=f'{OUT}/{name}'; os.makedirs(d,exist_ok=True)
    keys=KEYS[name]; gtf=f'{B}/gt/{name}.txt'; gt=open(gtf).read() if os.path.exists(gtf) else None
    res=[]; t0=time.time()
    a=load(path); H0,W0=a.shape[:2]
    def rec(var,fn,ocr_png=None,dims=None,note=''):
        size=save_bytes(fn)
        txt=ocr(ocr_png or fn,f'{name}/{var}')
        n,hit=keyscore(txt,keys)
        r=dict(img=name,var=var,bytes=size,dims=dims,keys=n,nkeys=len(keys),missing=[k for k in keys if k not in hit],cer=(cer(txt,gt) if gt else None),note=note)
        res.append(r)
    # a) original
    rec('a_original',path,dims=f'{W0}x{H0}')
    # b) 2500 color jpeg q85
    b=resize_long(a,2500); fn=f'{d}/b_c2500_q85.jpg'; Image.fromarray(b).save(fn,quality=85,optimize=True)
    rec('b_farbe_2500_jpg85',fn,dims=f'{b.shape[1]}x{b.shape[0]}')
    # c) gray 2500 jpeg q78
    g=cv2.cvtColor(b,cv2.COLOR_RGB2GRAY); fn=f'{d}/c_g2500_q78.jpg'; Image.fromarray(g).save(fn,quality=78,optimize=True)
    rec('c_grau_2500_jpg78',fn,dims=f'{g.shape[1]}x{g.shape[0]}')
    # d) document cleanup
    w,cropped=dewarp(a,name)
    if not cropped: w=trim(w)
    w=to_300dpi(w,DOC_TYPE.get(name))
    cc=clean_color(w); cg=bg_normalize(cv2.cvtColor(w,cv2.COLOR_RGB2GRAY))
    dims=f'{cg.shape[1]}x{cg.shape[0]}'; note=('crop-manuell' if name in MANUAL_QUAD else 'crop') if cropped else 'nocrop'
    if QUICK:
        png=f'{d}/d_clean_gray.png'; Image.fromarray(cg).save(png)
        fn=f'{d}/d_g_webp50.webp'; sh(f'cwebp -quiet -q 50 -m 6 -metadata none "{png}" -o "{fn}"'); p2=fn+'.png'; Image.open(fn).save(p2); rec('d_grau_clean_webp50',fn,ocr_png=p2,dims=dims,note=note)
        fn=f'{d}/d_g_avif30.avif'; sh(f'avifenc -q 30 -s 6 -y 400 "{png}" "{fn}"'); p2=fn+'.png'; sh(f'avifdec "{fn}" "{p2}"'); rec('d_grau_clean_avif30',fn,ocr_png=p2,dims=dims,note=note)
        bw=sauvola(cg); bpng=f'{d}/e_bw.png'; Image.fromarray(bw).convert('1').save(bpng,optimize=True); rec('e_bilevel_png',bpng,dims=dims,note=note)
        tif=f'{d}/e_bw_g4.tif'; Image.fromarray(bw).convert('1').save(tif,compression='group4'); rec('e_bilevel_tiff_g4',tif,ocr_png=bpng,dims=dims,note=note)
        json.dump(res,open(f'{d}/res.json','w'),indent=1); return res
    png=f'{d}/d_clean_gray.png'; Image.fromarray(cg).save(png)
    pngc=f'{d}/d_clean_color.png'; Image.fromarray(cc).save(pngc)
    for q in (75,60):
        fn=f'{d}/d_g_jpg{q}.jpg'; Image.fromarray(cg).save(fn,quality=q,optimize=True); rec(f'd_grau_clean_jpg{q}',fn,dims=dims,note=note)
    for q in (75,50,30):
        fn=f'{d}/d_g_webp{q}.webp'; sh(f'cwebp -quiet -q {q} -m 6 -metadata none "{png}" -o "{fn}"')
        p2=fn+'.png'; Image.open(fn).save(p2); rec(f'd_grau_clean_webp{q}',fn,ocr_png=p2,dims=dims,note=note)
    for q in (60,45,30):
        fn=f'{d}/d_g_avif{q}.avif'; sh(f'avifenc -q {q} -s 6 -y 400 --ignore-exif --ignore-xmp "{png}" "{fn}"')
        p2=fn+'.png'; sh(f'avifdec "{fn}" "{p2}"'); rec(f'd_grau_clean_avif{q}',fn,ocr_png=p2,dims=dims,note=note)
    fn=f'{d}/d_g_jxl.jxl'; sh(f'cjxl -q 70 -e 7 "{png}" "{fn}"'); p2=fn+'.png'; sh(f'djxl "{fn}" "{p2}"'); rec('d_grau_clean_jxl70',fn,ocr_png=p2,dims=dims,note=note)
    for q in (45,):
        fn=f'{d}/d_c_avif{q}.avif'; sh(f'avifenc -q {q} -s 6 -y 420 --ignore-exif --ignore-xmp "{pngc}" "{fn}"')
        p2=fn+'.png'; sh(f'avifdec "{fn}" "{p2}"'); rec(f'd_farbe_clean_avif{q}',fn,ocr_png=p2,dims=dims,note=note)
    fn=f'{d}/d_c_webp60.webp'; sh(f'cwebp -quiet -q 60 -m 6 -metadata none "{pngc}" -o "{fn}"'); p2=fn+'.png'; Image.open(fn).save(p2)
    rec('d_farbe_clean_webp60',fn,ocr_png=p2,dims=dims,note=note)
    fn=f'{d}/d_c_jpg70.jpg'; Image.fromarray(cc).save(fn,quality=70,optimize=True); rec('d_farbe_clean_jpg70',fn,dims=dims,note=note)
    # e) bilevel
    bw=sauvola(cg); bpng=f'{d}/e_bw.png'; Image.fromarray(bw).convert('1').save(bpng,optimize=True); sh(f'optipng -quiet -o2 "{bpng}"')
    rec('e_bilevel_png',bpng,dims=dims,note=note)
    tif=f'{d}/e_bw_g4.tif'; Image.fromarray(bw).convert('1').save(tif,compression='group4',dpi=(300,300)); rec('e_bilevel_tiff_g4',tif,ocr_png=bpng,dims=dims,note=note)
    pdf=f'{d}/e_bw_g4.pdf'; sh(f'img2pdf "{tif}" -o "{pdf}"'); rec('e_bilevel_pdf_ccitt',pdf,ocr_png=bpng,dims=dims,note=note)
    pdf=f'{d}/e_bw_jbig2.pdf'; pdf_jbig2(bpng,pdf,bw.shape[1],bw.shape[0],300); rec('e_bilevel_pdf_jbig2_lossless',pdf,ocr_png=bpng,dims=dims,note=note)
    # f) searchable PDFs
    src=f'{d}/d_g_jpg60.jpg'; Image.open(src).save(src,quality=60,dpi=(300,300))
    pdf=f'{d}/f_sandwich_gray.pdf'; sh(f'OMP_THREAD_LIMIT=1 tesseract "{src}" "{pdf[:-4]}" -l deu+eng --psm 4 pdf'); rec('f_pdf_text_grau_jpg60',pdf,ocr_png=src,dims=dims,note=note)
    pdf=f'{d}/f_sandwich_bw.pdf'; sh(f'OMP_THREAD_LIMIT=1 ocrmypdf -q --jobs 1 --image-dpi 300 -l deu+eng --optimize 2 --output-type pdf --tesseract-pagesegmode 4 "{bpng}" "{pdf}"'); rec('f_pdf_text_bilevel_jbig2',pdf,ocr_png=bpng,dims=dims,note=note)
    json.dump(res,open(f'{d}/res.json','w'),indent=1)
    print(name,'done',round(time.time()-t0),'s',flush=True)
    return res

if __name__=='__main__':
    from multiprocessing import Pool
    files=sorted(glob.glob(B+'/src/*.jpg')+glob.glob(B+'/src/*.png'))
    if len(sys.argv)>1: files=[f for f in files if any(x in f for x in sys.argv[1:])]
    with Pool(7) as p: allr=sum(p.map(run,files),[])
    json.dump(allr,open(OUT+'/results.json','w'),indent=1)
