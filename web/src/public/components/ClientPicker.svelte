<script lang="ts">
  import PixelIcon from '../../components/pixel/PixelIcon.svelte';
  import type { PublicClient, PublicView } from '../types';
  import { clientUsable, selectedOption } from '../utils';

  interface Props {
    clients: PublicClient[];
    view: PublicView;
    selectedIndex: number;
    selectedTargets: Record<string, string>;
    onselectclient: (index: number) => void;
    onselecttarget: (clientIndex: number, targetID: string) => void;
  }

  let {
    clients,
    view,
    selectedIndex,
    selectedTargets,
    onselectclient,
    onselecttarget,
  }: Props = $props();

  let openIndex = $state<number | null>(null);

  function selectClient(index: number) {
    if (!clientUsable(clients[index], view)) return;
    onselectclient(index);
    openIndex = clients[index].options.length > 1 && openIndex !== index ? index : null;
  }

  function selectTarget(event: MouseEvent, clientIndex: number, targetID: string) {
    event.stopPropagation();
    onselecttarget(clientIndex, targetID);
    openIndex = null;
  }
</script>

<section class="public-block client-section" aria-labelledby="client-heading">
  <div class="block-head">
    <h2 class="block-title" id="client-heading">选择客户端</h2>
    <span class="label">输出格式与下载目标</span>
  </div>
  <div class="client-picker">
    {#each clients as client, index (client.id)}
      {@const usable = clientUsable(client, view)}
      {@const target = selectedOption(client, selectedTargets[client.id], view)}
      <div class="client-slot" class:open={openIndex === index}>
        <button
          class="client-card"
          class:on={selectedIndex === index && usable}
          class:off={!usable}
          type="button"
          disabled={!usable}
          aria-expanded={client.options.length > 1 ? openIndex === index : undefined}
          onclick={() => selectClient(index)}
        >
          <img src={`static/icons/${client.icon}.svg`} width="32" height="32" alt="" aria-hidden="true" />
          <span class="ctext">
            <strong>{client.name}</strong>
            <small>{client.options.length > 1 ? `${target.name} · ${target.ext}` : `${target.ext} 格式`}</small>
          </span>
          <span class="client-state">
            <i></i>
            {#if client.options.length > 1}
              <PixelIcon name="chevron-down" size={10} />
            {/if}
          </span>
        </button>
        {#if client.options.length > 1 && openIndex === index}
          <div class="client-menu" role="radiogroup" aria-label={`${client.name} 格式`}>
            {#each client.options as option (option.id)}
              {@const optionEnabled = view === 'geosite' ? option.geosite : option.rules}
              <button
                type="button"
                role="radio"
                aria-checked={target.id === option.id}
                class:on={target.id === option.id}
                class:off={!optionEnabled}
                disabled={!optionEnabled}
                onclick={(event) => selectTarget(event, index, option.id)}
              >
                <span><strong>{option.name}</strong><small>{option.ext}</small></span>
                <i></i>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/each}
  </div>
</section>
