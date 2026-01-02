<script>
  import { onMount } from 'svelte';
  import ShareView from './components/ShareView.svelte';
  import ErrorView from './components/ErrorView.svelte';
  import Admin from './components/Admin.svelte';

  let token = '';
  let loading = true;
  let error = null;
  let shareInfo = null;
  let isAdminRoute = false;

  onMount(() => {
    const path = window.location.pathname;

    // Check if admin route
    if (path === '/admin' || path.startsWith('/admin/')) {
      isAdminRoute = true;
      loading = false;
      return;
    }

    // Extract token from URL path: /s/{token}
    const pathParts = path.split('/');
    const sIndex = pathParts.indexOf('s');
    if (sIndex !== -1 && pathParts[sIndex + 1]) {
      token = pathParts[sIndex + 1];
      loadShareInfo();
    } else {
      error = { message: 'Invalid share link', status: 400 };
      loading = false;
    }
  });

  async function loadShareInfo() {
    try {
      const response = await fetch(`/api/public/shares/${token}`);

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        if (response.status === 404) {
          error = { message: 'Share not found', status: 404 };
        } else if (response.status === 410) {
          error = { message: data.error || 'This share is no longer available', status: 410 };
        } else {
          error = { message: data.error || 'Failed to load share', status: response.status };
        }
      } else {
        shareInfo = await response.json();
      }
    } catch (e) {
      error = { message: 'Failed to connect to server', status: 500 };
    } finally {
      loading = false;
    }
  }

  function handlePasswordSuccess() {
    // Password validated - ShareView handles the state internally
  }
</script>

<main>
  {#if isAdminRoute}
    <Admin />
  {:else if loading}
    <div class="loading-container">
      <div class="spinner"></div>
      <p>Loading...</p>
    </div>
  {:else if error}
    <ErrorView {error} />
  {:else if shareInfo}
    <ShareView {shareInfo} {token} on:passwordSuccess={handlePasswordSuccess} />
  {/if}
</main>

<style>
  main {
    min-height: 100vh;
  }

  .loading-container {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
    gap: 1rem;
  }

  .loading-container p {
    color: #a0a0a0;
  }

  .spinner {
    width: 48px;
    height: 48px;
    border: 4px solid rgba(255,255,255,0.1);
    border-top-color: #00d4ff;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
