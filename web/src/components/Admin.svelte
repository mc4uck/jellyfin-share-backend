<script>
  import AdminLogin from './AdminLogin.svelte';
  import AdminDashboard from './AdminDashboard.svelte';

  let apiKey = localStorage.getItem('jfshare_admin_key') || '';
  let isLoggedIn = !!apiKey;

  function handleLogin(event) {
    apiKey = event.detail.apiKey;
    localStorage.setItem('jfshare_admin_key', apiKey);
    isLoggedIn = true;
  }

  function handleLogout() {
    apiKey = '';
    localStorage.removeItem('jfshare_admin_key');
    isLoggedIn = false;
  }
</script>

{#if isLoggedIn}
  <AdminDashboard {apiKey} on:logout={handleLogout} />
{:else}
  <AdminLogin on:login={handleLogin} />
{/if}
