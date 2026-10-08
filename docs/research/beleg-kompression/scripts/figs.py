import sys,os
sys.argv=['x']; exec(open('exp.py').read().split("if __name__")[0])
from PIL import ImageDraw, ImageFont
FD='/workspace/vc-reisekostenabrechnung/docs/research/beleg-kompression'; os.makedirs(FD,exist_ok=True)
fnt=ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf',22)
fs=ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf',18)
def geom(name):
    a=load(f'src/{name}.jpg'); w,c=dewarp(a,name)
    if not c: w=trim(w)
    return to_300dpi(w,DOC_TYPE.get(name))
def panel(arr,box,title,sub,W=560):
    x,y,w,h=box; im=Image.fromarray(arr[y:y+h,x:x+w]).convert('RGB')
    s=W/w; im=im.resize((W,int(h*s)),Image.LANCZOS if 'Bilevel' not in title else Image.NEAREST)
    c=Image.new('RGB',(W,im.height+70),'white'); c.paste(im,(0,70)); d=ImageDraw.Draw(c)
    d.text((8,6),title,font=fnt,fill=(0,0,0)); d.text((8,38),sub,font=fs,fill=(70,70,70)); return c
def row(panels,out):
    W=sum(p.width for p in panels)+10*(len(panels)-1); H=max(p.height for p in panels)
    c=Image.new('RGB',(W,H),(200,200,200)); x=0
    for p in panels: c.paste(p,(x,0)); x+=p.width+10
    c.save(out,optimize=True); print(out,c.size,os.path.getsize(out)//1024,'KB')
def kb(p): return f'{os.path.getsize(p)/1024:.0f} KB gesamt'
def arr(p): return np.array(Image.open(p).convert('RGB'))
# 1 Hotel / Stempel
n='16_hotel_a4_synth'; o=f'out/{n}'; box=(1000,1350,1400,900)
row([panel(geom(n),box,'Foto (entzerrt, Farbe)','Original: '+kb(f'src/{n}.jpg')),
     panel(arr(f'{o}/d_c_avif45.avif.png'),box,'Farbe bereinigt AVIF q45',kb(f'{o}/d_c_avif45.avif')),
     panel(arr(f'{o}/d_g_webp50.webp.png'),box,'Graustufen WebP q50',kb(f'{o}/d_g_webp50.webp')),
     panel(arr(f'{o}/e_bw.png'),box,'Bilevel (Sauvola) CCITT G4',kb(f'{o}/e_bw_g4.tif'))],f'{FD}/vergleich-stempel-hotelrechnung.png')
# 2 Aldi / Schatten
n='03_aldi_hesel'; o=f'out/{n}'; g=arr(f'{o}/d_clean_gray.png'); H=g.shape[0]; box=(0,int(H*0.70),945,int(H*0.22))
row([panel(geom(n),box,'Foto (entzerrt, Farbe)','Original: '+kb(f'src/{n}.jpg'),W=500),
     panel(arr(f'{o}/d_g_webp50.webp.png'),box,'Graustufen bereinigt WebP q50',kb(f'{o}/d_g_webp50.webp'),W=500),
     panel(arr(f'{o}/d_g_avif30.avif.png'),box,'Graustufen bereinigt AVIF q30',kb(f'{o}/d_g_avif30.avif'),W=500),
     panel(arr(f'{o}/e_bw.png'),box,'Bilevel CCITT G4',kb(f'{o}/e_bw_g4.tif'),W=500)],f'{FD}/vergleich-schatten-aldi.png')
# 3 verblasster Thermobon
ink=226; a=load(f'fade/ink{ink}.jpg'); w,_=dewarp(a); w=to_300dpi(w,'bon'); g=arr(f'fade/ink{ink}_gray_dec.png'); box=(0,int(g.shape[0]*0.3),945,int(g.shape[0]*0.45))
row([panel(w,box,'Foto (entzerrt)','extrem blasser Thermodruck (Kontrast ~12/255)',W=500),
     panel(g,box,'Graustufen bereinigt WebP q50',f"{os.path.getsize(f'fade/ink{ink}_gray.webp')/1024:.0f} KB",W=500),
     panel(arr(f'fade/ink{ink}_bw.png'),box,'Bilevel (Sauvola)','Schwelle fix, nicht nachjustierbar',W=500)],f'{FD}/vergleich-verblasster-thermobon.png')
# 4 Real zerknittert
n='12_real_oudepekela'; o=f'out/{n}'; g=arr(f'{o}/d_clean_gray.png'); H=g.shape[0]; box=(0,int(H*0.30),945,int(H*0.30))
row([panel(geom(n),box,'Foto (entzerrt, Farbe)','Original: '+kb(f'src/{n}.jpg'),W=500),
     panel(arr(f'{o}/d_g_webp50.webp.png'),box,'Graustufen bereinigt WebP q50',kb(f'{o}/d_g_webp50.webp'),W=500),
     panel(arr(f'{o}/e_bw.png'),box,'Bilevel CCITT G4',kb(f'{o}/e_bw_g4.tif'),W=500)],f'{FD}/vergleich-zerknittert-real.png')
