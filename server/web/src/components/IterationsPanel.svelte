<script lang="ts">
  import type { IterationSummary, IterationDetail } from '../lib/types';
  import { fetchStepIteration } from '../lib/api';
  import { httpStatusCategory, statusLabel } from '../lib/format';
  import JsonViewer from './JsonViewer.svelte';
  import HeadersTable from './HeadersTable.svelte';
  import OASValidationPanel from './OASValidationPanel.svelte';
  import LoadingSpinner from './LoadingSpinner.svelte';

  interface Props {
    runId: string;
    stepId: string;
    attempt?: number;
    iterations: IterationSummary[];
    repeatStop?: string;
  }

  let { runId, stepId, attempt, iterations, repeatStop }: Props = $props();

  let selected = $state<number | null>(null);
  let detail = $state<IterationDetail | null>(null);
  let loading = $state(false);
  let error = $state('');

  // select loads a request's detail, or closes it when it is already open.
  async function select(index: number) {
    if (selected === index) {
      selected = null;
      detail = null;
      return;
    }
    selected = index;
    detail = null;
    error = '';
    loading = true;
    try {
      const d = await fetchStepIteration(runId, stepId, index, attempt);
      if (selected === index) detail = d;
    } catch (e) {
      if (selected === index) error = e instanceof Error ? e.message : 'Failed to load the request';
    } finally {
      if (selected === index) loading = false;
    }
  }

  function onRowKey(e: KeyboardEvent, index: number) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      select(index);
    }
  }

  function outputsText(outputs?: Record<string, unknown>): string {
    if (!outputs) return '';
    return Object.keys(outputs)
      .sort()
      .map((name) => `${name} ${JSON.stringify(outputs[name])}`)
      .join(', ');
  }
</script>

<p style="margin-bottom: 0.75rem; font-size: 0.85rem;">
  {iterations.length} {iterations.length === 1 ? 'request' : 'requests'}{#if repeatStop}, stopped by <span class="badge badge-sm badge-muted">{repeatStop}</span>{/if}.
  The Request and Response tabs show the last one; select a request to see its own.
</p>

<table class="detail-table">
  <thead>
    <tr>
      <th style="width: 3rem;">#</th>
      <th>Status</th>
      <th>Duration</th>
      <th>Until</th>
      <th>Outputs</th>
    </tr>
  </thead>
  <tbody>
    {#each iterations as it (it.index)}
      <tr
        class="iteration-row"
        class:iteration-row-selected={selected === it.index}
        role="button"
        tabindex="0"
        aria-expanded={selected === it.index}
        onclick={() => select(it.index)}
        onkeydown={(e: KeyboardEvent) => onRowKey(e, it.index)}
      >
        <td class="dt-mono">{it.index}</td>
        <td>
          {#if it.status}
            <span class="step-status step-status-{httpStatusCategory(it.status)}">{statusLabel(it.status, it.grpcCode)}</span>
          {/if}
        </td>
        <td class="dt-mono">
          {it.durationDisplay}
          {#if it.retryCount}<span class="step-retry-badge">retried {it.retryCount}</span>{/if}
        </td>
        <td>{it.untilMet ? 'met' : ''}</td>
        <td class="dt-mono dt-muted">
          {#if it.error}{it.error}{:else}{outputsText(it.outputs)}{/if}
          {#if it.oasErrorCount}<span class="badge badge-sm badge-muted">OAS {it.oasErrorCount}</span>{/if}
        </td>
      </tr>
    {/each}
  </tbody>
</table>

{#if selected !== null}
  <div class="iteration-detail">
    {#if loading}
      <LoadingSpinner />
    {:else if error}
      <div class="error-message"><p>{error}</p></div>
    {:else if detail}
      <h4 class="section-heading">Request {detail.index} of {detail.count}</h4>
      {#if detail.error}
        <div class="step-detail-error">{detail.error}</div>
      {/if}
      {#if detail.request}
        <div class="http-method-url">
          {#if detail.request.protocol === 'grpc'}
            <span class="http-method">gRPC</span>
            <span class="http-url">{detail.request.rpc}</span>
            <span class="grpc-target">{detail.request.target}</span>
          {:else}
            <span class="http-method">{detail.request.method}</span>
            <span class="http-url">{detail.request.url}</span>
          {/if}
        </div>
        {#if detail.request.headers && detail.request.headers.length > 0}
          <details>
            <summary class="section-heading collapsible-heading">Request headers ({detail.request.headers.length})</summary>
            <HeadersTable headers={detail.request.headers} />
          </details>
        {/if}
        {#if detail.request.formFields && detail.request.formFields.length > 0}
          <h4 class="section-heading">Request body ({detail.request.formFields.length} form fields)</h4>
          <HeadersTable headers={detail.request.formFields} />
        {:else if detail.request.body !== undefined && detail.request.body !== null}
          <h4 class="section-heading">Request body</h4>
          <JsonViewer data={detail.request.body} />
        {/if}
      {/if}
      {#if detail.inputs}
        <h4 class="section-heading">Inputs sent</h4>
        <JsonViewer data={detail.inputs} />
      {/if}
      {#if detail.response}
        <h4 class="section-heading">
          Response
          <span class="step-status step-status-{httpStatusCategory(detail.response.status)}">{statusLabel(detail.response.status, detail.response.grpcCode)}</span>
        </h4>
        {#if detail.response.headers && detail.response.headers.length > 0}
          <details>
            <summary class="section-heading collapsible-heading">Response headers ({detail.response.headers.length})</summary>
            <HeadersTable headers={detail.response.headers} />
          </details>
        {/if}
        {#if detail.response.body !== undefined && detail.response.body !== null}
          <JsonViewer data={detail.response.body} />
        {/if}
      {/if}
      {#if detail.outputs}
        <h4 class="section-heading">Outputs</h4>
        <JsonViewer data={detail.outputs} />
      {/if}
      {#if detail.oasValidation}
        <h4 class="section-heading">OAS</h4>
        <OASValidationPanel oasValidation={detail.oasValidation} />
      {/if}
    {/if}
  </div>
{/if}

<style>
  .iteration-row {
    cursor: pointer;
  }
  .iteration-row:hover,
  .iteration-row:focus-visible,
  .iteration-row-selected {
    background: var(--color-surface-hover, rgba(127, 127, 127, 0.12));
  }
  .iteration-detail {
    margin-top: 1rem;
  }
</style>
