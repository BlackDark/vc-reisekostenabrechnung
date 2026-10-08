import { render } from "solid-js/web"
import { createSignal } from "solid-js"
import "./index.css"
import { Button } from "~/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "~/components/ui/dialog"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "~/components/ui/select"

function App() {
  const [rate, setRate] = createSignal<string | null>("19 %")
  return (
    <main class="p-4 space-y-4">
      <h1 class="text-xl font-bold">Reisekosten</h1>
      <Select value={rate()} onChange={setRate} options={["0 %", "7 %", "19 %"]} placeholder="MwSt"
        itemComponent={(p) => <SelectItem item={p.item}>{p.item.rawValue}</SelectItem>}>
        <SelectTrigger class="w-40"><SelectValue<string>>{(s) => s.selectedOption()}</SelectValue></SelectTrigger>
        <SelectContent />
      </Select>
      <Dialog>
        <DialogTrigger as={Button}>Neue Reise</DialogTrigger>
        <DialogContent>
          <DialogHeader><DialogTitle>Neue Reise</DialogTitle><DialogDescription>MwSt: {rate()}</DialogDescription></DialogHeader>
          <Button variant="secondary">Speichern</Button>
        </DialogContent>
      </Dialog>
    </main>
  )
}
render(() => <App />, document.getElementById("root")!)
