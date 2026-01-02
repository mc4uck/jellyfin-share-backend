<script>
  import { createEventDispatcher } from 'svelte';

  const dispatch = createEventDispatcher();

  let apiKey = '';
  let error = '';
  let loading = false;

  async function login() {
    if (!apiKey.trim()) {
      error = 'Please enter an API key';
      return;
    }

    loading = true;
    error = '';

    try {
      const response = await fetch('/api/admin/stats', {
        headers: { 'X-Backend-Key': apiKey }
      });

      if (response.ok) {
        dispatch('login', { apiKey });
      } else {
        error = 'Invalid API key';
      }
    } catch (e) {
      error = 'Failed to connect to server';
    } finally {
      loading = false;
    }
  }
</script>

<div class="login-container">
  <div class="login-box">
    <h1>Admin Login</h1>
    <p class="subtitle">Jellyfin Share Management</p>

    <form on:submit|preventDefault={login}>
      <div class="input-group">
        <label for="apiKey">Backend API Key</label>
        <input
          type="password"
          id="apiKey"
          bind:value={apiKey}
          placeholder="Enter API key"
          disabled={loading}
        />
      </div>

      {#if error}
        <p class="error">{error}</p>
      {/if}

      <button type="submit" disabled={loading}>
        {#if loading}
          <span class="spinner"></span>
        {:else}
          Login
        {/if}
      </button>
    </form>
  </div>
</div>

<style>
  .login-container {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
    padding: 1rem;
  }

  .login-box {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    padding: 2.5rem;
    width: 100%;
    max-width: 400px;
    backdrop-filter: blur(10px);
  }

  h1 {
    margin: 0 0 0.5rem 0;
    font-size: 1.75rem;
    color: #fff;
    text-align: center;
  }

  .subtitle {
    color: rgba(255, 255, 255, 0.5);
    text-align: center;
    margin: 0 0 2rem 0;
    font-size: 0.9rem;
  }

  .input-group {
    margin-bottom: 1.5rem;
  }

  label {
    display: block;
    color: rgba(255, 255, 255, 0.7);
    font-size: 0.85rem;
    margin-bottom: 0.5rem;
  }

  input {
    width: 100%;
    padding: 0.875rem 1rem;
    background: rgba(0, 0, 0, 0.3);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 8px;
    color: #fff;
    font-size: 1rem;
    transition: all 0.2s;
    box-sizing: border-box;
  }

  input:focus {
    outline: none;
    border-color: #00d4ff;
    background: rgba(0, 0, 0, 0.4);
  }

  input::placeholder {
    color: rgba(255, 255, 255, 0.3);
  }

  button {
    width: 100%;
    padding: 1rem;
    background: linear-gradient(135deg, #00d4ff, #0099cc);
    border: none;
    border-radius: 8px;
    color: #000;
    font-size: 1rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
  }

  button:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 8px 20px rgba(0, 212, 255, 0.3);
  }

  button:disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }

  .error {
    color: #ff6b6b;
    font-size: 0.85rem;
    margin: 0 0 1rem 0;
    text-align: center;
  }

  .spinner {
    width: 20px;
    height: 20px;
    border: 2px solid rgba(0, 0, 0, 0.2);
    border-top-color: #000;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
