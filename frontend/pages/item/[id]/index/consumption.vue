<script setup lang="ts">
  import type { ItemOut } from "~~/lib/api/types/data-contracts";

  const props = defineProps<{
    item: ItemOut;
  }>();
</script>

<template>
  <BaseContainer class="mb-6 flex flex-col gap-8">
    <PantryConsumptionView :item-id="props.item.id" :quantity="props.item.quantity" />
  </BaseContainer>
</template>
