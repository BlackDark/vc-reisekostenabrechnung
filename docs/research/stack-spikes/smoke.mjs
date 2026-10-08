import { chromium } from "playwright"
const url = process.argv[2]
const b = await chromium.launch(); const p = await b.newPage()
const errs = []; p.on("pageerror", e => errs.push("pageerror: " + e.message)); p.on("console", m => m.type()==="error" && errs.push("console: " + m.text()))
await p.goto(url); await p.waitForTimeout(800)
const r = {}
try { r.h1 = await p.locator("h1").innerText({timeout:2000}) } catch(e){ r.h1="FAIL" }
try { await p.getByRole("button", {name:"Neue Reise"}).click({timeout:2000}); await p.waitForTimeout(400); r.dialog = await p.getByRole("dialog").isVisible() } catch(e){ r.dialog="FAIL "+e.message.split("\n")[0] }
try { await p.keyboard.press("Escape"); await p.waitForTimeout(300); r.dialogClosed = !(await p.getByRole("dialog").isVisible()) } catch(e){ r.dialogClosed="FAIL" }
try { await p.locator("button[aria-haspopup=listbox], [role=combobox]").first().click({timeout:2000}); await p.waitForTimeout(300); await p.getByRole("option",{name:"7 %"}).click({timeout:2000}); await p.waitForTimeout(300); r.selectText = await p.locator("main").innerText() } catch(e){ r.select="FAIL "+e.message.split("\n")[0] }
console.log(JSON.stringify(r), "\nerrors:", errs.slice(0,5))
await b.close()
