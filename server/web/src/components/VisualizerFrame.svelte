<script lang="ts">
  interface Props {
    visualizerId: string;
    responseBody: unknown;
  }

  let { visualizerId, responseBody }: Props = $props();
  let iframeEl = $state<HTMLIFrameElement | null>(null);
  let frameHeight = $state(400);

  function sendData() {
    if (!iframeEl?.contentWindow) return;

    // Forward the theme under the variable names visualizers are documented to
    // receive (docs/user/visualizers.md), read from the app's own variables.
    const style = getComputedStyle(document.documentElement);
    const theme: Record<string, string> = {};
    const contract: [name: string, source: string][] = [
      ['--color-bg', '--color-bg'],
      ['--color-surface', '--color-surface'],
      ['--color-text', '--color-text'],
      ['--color-text-secondary', '--color-text-muted'],
      ['--color-text-muted', '--color-text-muted'],
      ['--color-border', '--color-border'],
      ['--color-primary', '--color-primary'],
      ['--color-success', '--color-success'],
      ['--color-warning', '--color-warning'],
      ['--color-danger', '--color-error'],
      ['--font-mono', '--font-mono'],
      ['--font-sans', '--font-sans'],
    ];
    for (const [name, source] of contract) {
      const val = style.getPropertyValue(source).trim();
      if (val) theme[name] = val;
    }

    // Deep-clone to strip Svelte 5 reactivity proxies — postMessage's
    // structured clone algorithm cannot handle Proxy objects.
    const plainBody = JSON.parse(JSON.stringify(responseBody));

    iframeEl.contentWindow.postMessage(
      { type: 'aat-visualizer-data', body: plainBody, theme },
      '*',
    );
  }

  $effect(() => {
    function onMessage(e: MessageEvent) {
      if (!e.data || typeof e.data !== 'object') return;

      if (e.data.type === 'aat-visualizer-ready') {
        sendData();
      } else if (e.data.type === 'aat-visualizer-resize' && typeof e.data.height === 'number') {
        frameHeight = Math.max(100, Math.min(e.data.height, 10000));
      }
    }

    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  });
</script>

<div class="visualizer-frame-wrapper">
  <iframe
    bind:this={iframeEl}
    src="/api/visualizers/{visualizerId}"
    sandbox="allow-scripts"
    title="Visualizer"
    style="width: 100%; height: {frameHeight}px; border: 1px solid var(--color-border, #333); border-radius: 6px;"
  ></iframe>
</div>

<style>
  .visualizer-frame-wrapper {
    margin-top: 0.5rem;
  }
</style>
