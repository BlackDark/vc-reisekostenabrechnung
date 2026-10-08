import sys,os,json
sys.argv=['x']; exec(open('exp.py').read().split("if __name__")[0])
import glob
from multiprocessing import Pool
def one(path):
    name=os.path.splitext(os.path.basename(path))[0]; d=f'out/{name}'; keys=KEYS[name]; res=[]
    a=load(path); w,c=dewarp(a,name)
    if not c: w=trim(w)
    w=to_300dpi(w,DOC_TYPE.get(name)); dims=f'{w.shape[1]}x{w.shape[0]}'
    def rec(var,fn,png=None):
        t=ocr(png or fn,f'{name}/{var}'); res.append(dict(img=name,var=var,bytes=os.path.getsize(fn),keys=keyscore(t,keys)[0],nkeys=len(keys),dims=dims))
    fn=f'{d}/x_raw_c_jpg85.jpg'; Image.fromarray(w).save(fn,quality=85,optimize=True); rec('x_entzerrt_farbe_roh_jpg85',fn)
    fn=f'{d}/x_raw_c_jpg75.jpg'; Image.fromarray(w).save(fn,quality=75,optimize=True); rec('x_entzerrt_farbe_roh_jpg75',fn)
    pngc=f'{d}/d_clean_color.png'
    fn=f'{d}/x_c_webp50.webp'; sh(f'cwebp -quiet -q 50 -m 6 "{pngc}" -o "{fn}"'); p=fn+'.png'; Image.open(fn).save(p); rec('x_farbe_clean_webp50',fn,p)
    fn=f'{d}/x_c_avif30.avif'; sh(f'avifenc -q 30 -s 6 -y 420 "{pngc}" "{fn}"'); p=fn+'.png'; sh(f'avifdec "{fn}" "{p}"'); rec('x_farbe_clean_avif30',fn,p)
    # thumbnail
    t=Image.open(pngc); t.thumbnail((320,320)); fn=f'{d}/x_thumb.webp'; t.save(fn,'WEBP',quality=70); res.append(dict(img=name,var='x_thumb_webp_320',bytes=os.path.getsize(fn),keys=0,nkeys=len(keys),dims=f'{t.width}x{t.height}'))
    # OCR text size
    txt=f'out/ocr/{name}/d_grau_clean_webp50.txt'; res.append(dict(img=name,var='x_ocr_text_utf8',bytes=os.path.getsize(txt),keys=0,nkeys=len(keys),dims=''))
    # timing: tesseract single thread on cleaned gray
    import time; t0=time.time(); sh(f'OMP_THREAD_LIMIT=1 tesseract {d}/d_clean_gray.png /tmp/tt_{name} -l deu+eng --psm 4'); res.append(dict(img=name,var='x_tesseract_sekunden',bytes=round(time.time()-t0,2),keys=0,nkeys=0,dims=dims))
    return res
files=sorted(glob.glob('src/*.jpg')+glob.glob('src/*.png'))
with Pool(7) as p: R=sum(p.map(one,files),[])
json.dump(R,open('out/results_extra.json','w'),indent=1)
import statistics as st
vs=[];[vs.append(r['var']) for r in R if r['var'] not in vs]
for v in vs:
    rs=[r for r in R if r['var']==v]; f=1 if 'sekunden' in v else 1/1024
    print(f"{v:30} med {st.median(r['bytes']*f for r in rs):7.2f} mean {st.mean(r['bytes']*f for r in rs):7.2f} keys {sum(r['keys'] for r in rs)}")
