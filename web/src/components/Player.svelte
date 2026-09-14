<script>
  import { onMount, onDestroy, createEventDispatcher } from "svelte";
  import Hls from "hls.js";

  export let playbackData;
  export let title;
  export let isAudio = false;
  export let hasPrevious = false;
  export let hasNext = false;

  const dispatch = createEventDispatcher();

  let mediaElement;
  let hls;
  let heartbeatInterval;
  let error = null;
  let isFullscreen = false;
  let cleanedUp = false;

  onMount(() => {
    initPlayer();
    startHeartbeat();
    document.addEventListener("fullscreenchange", handleFullscreenChange);

    return () => {
      document.removeEventListener("fullscreenchange", handleFullscreenChange);
    };
  });

  onDestroy(() => {
    cleanup();
  });

  function initPlayer() {
    if (!playbackData?.playbackUrl) {
      error = "No playback URL provided";
      return;
    }

    if (Hls.isSupported()) {
      hls = new Hls({
        enableWorker: true,
        lowLatencyMode: false,
        backBufferLength: 90,
      });

      hls.loadSource(playbackData.playbackUrl);
      hls.attachMedia(mediaElement);

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        mediaElement.play().catch((e) => {
          console.log("Autoplay prevented:", e);
        });
      });

      hls.on(Hls.Events.ERROR, (event, data) => {
        if (data.fatal) {
          switch (data.type) {
            case Hls.ErrorTypes.NETWORK_ERROR:
              error = "Network error - trying to recover...";
              hls.startLoad();
              break;
            case Hls.ErrorTypes.MEDIA_ERROR:
              error = "Media error - trying to recover...";
              hls.recoverMediaError();
              break;
            default:
              error = "Playback error occurred";
              cleanup();
              break;
          }
        }
      });
    } else if (mediaElement.canPlayType("application/vnd.apple.mpegurl")) {
      // Safari native HLS support
      mediaElement.src = playbackData.playbackUrl;
      mediaElement.addEventListener("loadedmetadata", () => {
        mediaElement.play().catch((e) => {
          console.log("Autoplay prevented:", e);
        });
      });
    } else {
      error = "HLS playback is not supported in this browser";
    }
  }

  function startHeartbeat() {
    heartbeatInterval = setInterval(async () => {
      if (!playbackData?.sessionId) return;

      try {
        const response = await fetch(
          `/api/public/sessions/${playbackData.sessionId}/heartbeat`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              positionSeconds: Math.floor(mediaElement?.currentTime || 0),
            }),
            credentials: "include",
          },
        );

        if (response.ok) {
          const data = await response.json();
          if (data.status !== "ok") {
            error = data.message || "Session ended";
            cleanup();
          }
        }
      } catch (e) {
        console.error("Heartbeat failed:", e);
      }
    }, 15000); // Every 15 seconds
  }

  async function cleanup() {
    if (cleanedUp) return;
    cleanedUp = true;

    if (heartbeatInterval) {
      clearInterval(heartbeatInterval);
      heartbeatInterval = null;
    }

    if (hls) {
      hls.destroy();
      hls = null;
    }

    if (playbackData?.sessionId) {
      try {
        await fetch(`/api/public/sessions/${playbackData.sessionId}/finish`, {
          method: "POST",
          credentials: "include",
        });
      } catch (e) {
        console.error("Failed to notify session end:", e);
      }
    }
  }

  async function handleClose() {
    await cleanup();
    dispatch("close");
  }

  async function handleEnded() {
    await cleanup();
    dispatch("ended");
  }

  async function handlePrevious() {
    if (!hasPrevious) return;
    await cleanup();
    dispatch("previous");
  }

  async function handleNext() {
    if (!hasNext) return;
    await cleanup();
    dispatch("next");
  }

  function toggleFullscreen() {
    if (isAudio) return;
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  }

  function handleFullscreenChange() {
    isFullscreen = !!document.fullscreenElement;
  }

  function handleKeydown(event) {
    switch (event.key) {
      case "Escape":
        if (!isFullscreen) {
          handleClose();
        }
        break;
      case " ":
        event.preventDefault();
        if (mediaElement.paused) {
          mediaElement.play();
        } else {
          mediaElement.pause();
        }
        break;
      case "f":
        toggleFullscreen();
        break;
      case "ArrowLeft":
        mediaElement.currentTime -= 10;
        break;
      case "ArrowRight":
        mediaElement.currentTime += 10;
        break;
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="player-container">
  <div class="player-header">
    <h2>{title}</h2>
    <button class="close-button" on:click={handleClose}>
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <path d="M18 6L6 18M6 6l12 12" />
      </svg>
    </button>
  </div>

  <div class="video-wrapper">
    {#if error}
      <div class="error-overlay">
        <p>{error}</p>
        <button on:click={handleClose}>Go Back</button>
      </div>
    {/if}

    {#if isAudio}
      <div class="audio-player">
        <div class="audio-art" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 3v10.55A4 4 0 1 0 14 17V7h4V3h-6z" />
          </svg>
        </div>
        <audio bind:this={mediaElement} controls autoplay on:ended={handleEnded}
        ></audio>
      </div>
    {:else}
      <video
        bind:this={mediaElement}
        controls
        playsinline
        autoplay
        on:ended={handleEnded}
      >
        <track kind="captions" />
      </video>
    {/if}
  </div>

  <div class="player-controls">
    {#if isAudio}
      <button
        on:click={handlePrevious}
        title="Previous track"
        disabled={!hasPrevious}
      >
        <svg viewBox="0 0 24 24" fill="currentColor">
          <path d="M6 6h2v12H6zm3.5 6 8.5 6V6z" />
        </svg>
      </button>
      <button on:click={handleNext} title="Next track" disabled={!hasNext}>
        <svg viewBox="0 0 24 24" fill="currentColor">
          <path d="M16 6h2v12h-2zM6 18l8.5-6L6 6z" />
        </svg>
      </button>
    {:else}
      <button on:click={toggleFullscreen} title="Toggle fullscreen (F)">
        {#if isFullscreen}
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path
              d="M5 16h3v3h2v-5H5v2zm3-8H5v2h5V5H8v3zm6 11h2v-3h3v-2h-5v5zm2-11V5h-2v5h5V8h-3z"
            />
          </svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path
              d="M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z"
            />
          </svg>
        {/if}
      </button>
    {/if}
  </div>
</div>

<style>
  .player-container {
    position: fixed;
    inset: 0;
    background: #000;
    z-index: 1000;
    display: flex;
    flex-direction: column;
  }

  .player-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    background: linear-gradient(
      to bottom,
      rgba(0, 0, 0, 0.8) 0%,
      transparent 100%
    );
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    z-index: 10;
  }

  .player-header h2 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0;
    color: #fff;
    text-shadow: 0 2px 4px rgba(0, 0, 0, 0.5);
  }

  .close-button {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    border: none;
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.2s;
  }

  .close-button:hover {
    background: rgba(255, 255, 255, 0.2);
  }

  .close-button svg {
    width: 24px;
    height: 24px;
  }

  .video-wrapper {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
  }

  video {
    width: 100%;
    height: 100%;
    max-height: 100vh;
    background: #000;
  }

  .audio-player {
    width: min(640px, 90vw);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2rem;
    padding: 2rem;
  }

  .audio-art {
    width: 180px;
    height: 180px;
    border-radius: 24px;
    background: rgba(255, 255, 255, 0.08);
    display: flex;
    align-items: center;
    justify-content: center;
    color: rgba(255, 255, 255, 0.75);
  }

  .audio-art svg {
    width: 96px;
    height: 96px;
  }

  audio {
    width: 100%;
  }

  .error-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background: rgba(0, 0, 0, 0.9);
    color: #ff6b6b;
    gap: 1rem;
  }

  .error-overlay button {
    padding: 0.75rem 1.5rem;
    background: #333;
    color: #fff;
    border: none;
    border-radius: 8px;
    cursor: pointer;
  }

  .player-controls {
    position: absolute;
    bottom: 80px;
    right: 1rem;
    z-index: 10;
  }

  .player-controls button {
    width: 44px;
    height: 44px;
    border-radius: 50%;
    border: none;
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.2s;
  }

  .player-controls button:hover {
    background: rgba(255, 255, 255, 0.2);
  }

  .player-controls button:disabled {
    opacity: 0.35;
    cursor: default;
  }

  .player-controls svg {
    width: 24px;
    height: 24px;
  }
</style>
