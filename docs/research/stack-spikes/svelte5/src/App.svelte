<script lang="ts">
  import { Button } from "$lib/components/ui/button"
  import * as Dialog from "$lib/components/ui/dialog"
  import * as Select from "$lib/components/ui/select"
  let rate = $state("19 %")
  const rates = ["0 %", "7 %", "19 %"]
</script>
<main class="p-4 space-y-4">
  <h1 class="text-xl font-bold">Reisekosten</h1>
  <Select.Root type="single" bind:value={rate}>
    <Select.Trigger class="w-40">{rate}</Select.Trigger>
    <Select.Content>
      {#each rates as r (r)}<Select.Item value={r} label={r}>{r}</Select.Item>{/each}
    </Select.Content>
  </Select.Root>
  <Dialog.Root>
    <Dialog.Trigger>{#snippet child({ props })}<Button {...props}>Neue Reise</Button>{/snippet}</Dialog.Trigger>
    <Dialog.Content>
      <Dialog.Header><Dialog.Title>Neue Reise</Dialog.Title><Dialog.Description>MwSt: {rate}</Dialog.Description></Dialog.Header>
      <Button variant="secondary">Speichern</Button>
    </Dialog.Content>
  </Dialog.Root>
</main>
