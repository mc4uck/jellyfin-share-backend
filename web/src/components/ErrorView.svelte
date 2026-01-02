<script>
  export let error;

  const errorMessages = {
    404: {
      title: 'Not Found',
      icon: '🔍',
      description: 'This share link doesn\'t exist or may have been removed.'
    },
    410: {
      title: 'No Longer Available',
      icon: '⏰',
      description: 'This share has expired or been revoked.'
    },
    403: {
      title: 'Access Denied',
      icon: '🚫',
      description: 'You don\'t have permission to view this content.'
    },
    default: {
      title: 'Something Went Wrong',
      icon: '❌',
      description: 'An error occurred while loading this share.'
    }
  };

  $: errorInfo = errorMessages[error?.status] || errorMessages.default;
</script>

<div class="error-container">
  <div class="error-card">
    <div class="error-icon">{errorInfo.icon}</div>
    <h1>{errorInfo.title}</h1>
    <p class="description">{errorInfo.description}</p>
    {#if error?.message && error.message !== errorInfo.description}
      <p class="detail">{error.message}</p>
    {/if}
  </div>
</div>

<style>
  .error-container {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
    padding: 1rem;
  }

  .error-card {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    padding: 3rem;
    text-align: center;
    max-width: 400px;
  }

  .error-icon {
    font-size: 4rem;
    margin-bottom: 1rem;
  }

  h1 {
    font-size: 1.5rem;
    margin-bottom: 0.5rem;
    color: #fff;
  }

  .description {
    color: #a0a0a0;
    margin-bottom: 1rem;
  }

  .detail {
    color: #888;
    font-size: 0.875rem;
    padding-top: 1rem;
    border-top: 1px solid rgba(255, 255, 255, 0.1);
  }
</style>
