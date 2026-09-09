<script lang="ts">
  // 引用关系标识：中央方块是当前规则，左箭头=引用了别的规则，右箭头=被别的规则引用。
  // 画在 32×32 网格上、全部坐标取偶数，16px 渲染时恰好 2:1 缩放保持锐利。
  interface Props {
    direction: 'left' | 'right' | 'both';
    size?: number;
  }

  let { direction, size = 16 }: Props = $props();
  const left = $derived(direction === 'left' || direction === 'both');
  const right = $derived(direction === 'right' || direction === 'both');
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 32 32"
  fill="none"
  shape-rendering="crispEdges"
  aria-hidden="true"
>
  {#if left}
    <rect x="8" y="14" width="4" height="4" fill="#83b7b4" />
  {:else}
    <rect x="4" y="14" width="8" height="4" fill="#96958f" />
  {/if}
  {#if right}
    <rect x="20" y="14" width="4" height="4" fill="#90bb75" />
  {:else}
    <rect x="20" y="14" width="8" height="4" fill="#96958f" />
  {/if}
  {#if left}
    <path d="M0 14H2V12H4V10H6V8H8V24H6V22H4V20H2V18H0Z" fill="#454545" />
    <path d="M2 14H4V12H6V10H8V22H6V20H4V18H2Z" fill="#83b7b4" />
  {/if}
  {#if right}
    <path d="M32 14H30V12H28V10H26V8H24V24H26V22H28V20H30V18H32Z" fill="#454545" />
    <path d="M30 14H28V12H26V10H24V22H26V20H28V18H30Z" fill="#90bb75" />
  {/if}
  <rect x="12" y="12" width="8" height="8" fill="#454545" />
  <rect x="14" y="14" width="4" height="4" fill="#eee9dc" />
  <rect x="14" y="14" width="4" height="2" fill="#fffaf0" />
</svg>
