<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import Hls from 'hls.js';

  export let playbackData;
  export let title;
  export let isAudio = false;
  export let hasPrevious = false;
  export let hasNext = false;
  export let coverUrl = '';
  export let artist = '';
  export let subtitle = '';
  export let currentIndex = -1;
  export let totalItems = 0;
  export let shuffleEnabled = false;
  export let repeatMode = 'off';

  const dispatch = createEventDispatcher();

  let mediaElement;
  let hls;
  let heartbeatInterval;
  let error = null;
  let isFullscreen = false;
  let cleanedUp = false;

  // Custom audio-player state.
  let currentTime = 0;
  let duration = 0;
  let isPaused = true;
  let volume = 1;

  $: progressPercent = duration > 0 ? Math.min(100, Math.max(0, (currentTime / duration) * 100)) : 0;
  $: volumePercent = Math.round(volume * 100);

  onMount(() => {
    // Keep audio volume between tracks. Player is recreated for every track
    // because ShareView keys it by playbackData.sessionId, so component-local
    // state would otherwise reset to 100% on each transition.
    if (isAudio) {
      try {
        const savedVolume = Number(localStorage.getItem('jfshare-audio-volume'));
        if (Number.isFinite(savedVolume) && savedVolume >= 0 && savedVolume <= 1) {
          volume = savedVolume;
        }
      } catch (e) {
        console.warn('Failed to restore audio volume:', e);
      }
    }

    initPlayer();
    startHeartbeat();
    document.addEventListener('fullscreenchange', handleFullscreenChange);

    return () => {
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
    };
  });

  onDestroy(() => {
    cleanup();
  });

  function initPlayer() {
    if (!playbackData?.playbackUrl) {
      error = 'No playback URL provided';
      return;
    }

    // Jellyfin's /Audio/{id}/universal endpoint returns the playable media
    // response itself (or a redirect to it), not an m3u8 manifest. Feed audio
    // directly to the native <audio> element. This also lets the browser use
    // byte-range seeking when the upstream response advertises Accept-Ranges.
    if (isAudio) {
      mediaElement.src = playbackData.playbackUrl;
      mediaElement.volume = volume;

      mediaElement.addEventListener('loadedmetadata', () => {
        duration = Number.isFinite(mediaElement.duration) ? mediaElement.duration : 0;
        mediaElement.play().catch(e => {
          console.log('Autoplay prevented:', e);
        });
      }, { once: true });

      mediaElement.addEventListener('error', () => {
        const code = mediaElement?.error?.code;
        error = code ? `Audio playback error (${code})` : 'Audio playback error';
      }, { once: true });

      // load() makes the browser fetch metadata immediately instead of waiting
      // for another state transition after Svelte mounted the element.
      mediaElement.load();
      return;
    }

    // Video continues to use the existing HLS path.
    if (Hls.isSupported()) {
      hls = new Hls({
        enableWorker: true,
        lowLatencyMode: false,
        backBufferLength: 90
      });

      hls.loadSource(playbackData.playbackUrl);
      hls.attachMedia(mediaElement);

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        mediaElement.volume = volume;
        if (Number.isFinite(mediaElement.duration)) {
          duration = mediaElement.duration;
        }
        mediaElement.play().catch(e => {
          console.log('Autoplay prevented:', e);
        });
      });

      hls.on(Hls.Events.ERROR, (event, data) => {
        if (data.fatal) {
          switch (data.type) {
            case Hls.ErrorTypes.NETWORK_ERROR:
              error = 'Network error - trying to recover...';
              hls.startLoad();
              break;
            case Hls.ErrorTypes.MEDIA_ERROR:
              error = 'Media error - trying to recover...';
              hls.recoverMediaError();
              break;
            default:
              error = 'Playback error occurred';
              cleanup();
              break;
          }
        }
      });
    } else if (mediaElement.canPlayType('application/vnd.apple.mpegurl')) {
      // Safari native HLS support for video.
      mediaElement.src = playbackData.playbackUrl;
      mediaElement.addEventListener('loadedmetadata', () => {
        mediaElement.volume = volume;
        duration = Number.isFinite(mediaElement.duration) ? mediaElement.duration : 0;
        mediaElement.play().catch(e => {
          console.log('Autoplay prevented:', e);
        });
      });
    } else {
      error = 'HLS playback is not supported in this browser';
    }
  }

  function startHeartbeat() {
    heartbeatInterval = setInterval(async () => {
      if (!playbackData?.sessionId) return;

      try {
        const response = await fetch(`/api/public/sessions/${playbackData.sessionId}/heartbeat`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            positionSeconds: Math.floor(mediaElement?.currentTime || 0)
          }),
          credentials: 'include'
        });

        if (response.ok) {
          const data = await response.json();
          if (data.status !== 'ok') {
            error = data.message || 'Session ended';
            cleanup();
          }
        }
      } catch (e) {
        console.error('Heartbeat failed:', e);
      }
    }, 15000);
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

    if (mediaElement) {
      mediaElement.pause();
    }

    if (playbackData?.sessionId) {
      try {
        await fetch(`/api/public/sessions/${playbackData.sessionId}/finish`, {
          method: 'POST',
          credentials: 'include'
        });
      } catch (e) {
        console.error('Failed to notify session end:', e);
      }
    }
  }

  async function handleClose() {
    await cleanup();
    dispatch('close');
  }

  async function handleEnded() {
    await cleanup();
    dispatch('ended');
  }

  async function handlePrevious() {
    if (!hasPrevious) return;
    await cleanup();
    dispatch('previous');
  }

  async function handleNext() {
    if (!hasNext) return;
    await cleanup();
    dispatch('next');
  }

  function toggleShuffle() {
    if (!isAudio) return;
    dispatch('toggleshuffle');
  }

  function toggleRepeat() {
    if (!isAudio) return;
    dispatch('togglerepeat');
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

  function togglePlayPause() {
    if (!mediaElement) return;
    if (mediaElement.paused) {
      mediaElement.play().catch(() => {});
    } else {
      mediaElement.pause();
    }
  }

  function handleTimeUpdate() {
    if (!mediaElement) return;
    currentTime = mediaElement.currentTime || 0;
    if (!duration && Number.isFinite(mediaElement.duration)) {
      duration = mediaElement.duration;
    }
  }

  function handleLoadedMetadata() {
    if (!mediaElement) return;
    duration = Number.isFinite(mediaElement.duration) ? mediaElement.duration : 0;
  }

  function handleSeek(event) {
    if (!mediaElement) return;
    const newTime = Number(event.currentTarget.value);
    if (Number.isFinite(newTime)) {
      mediaElement.currentTime = newTime;
      currentTime = newTime;
    }
  }

  function handleVolume(event) {
    volume = Number(event.currentTarget.value);

    if (mediaElement) {
      mediaElement.volume = volume;
    }

    if (isAudio) {
      try {
        localStorage.setItem('jfshare-audio-volume', String(volume));
      } catch (e) {
        console.warn('Failed to save audio volume:', e);
      }
    }
  }

  function formatClock(seconds) {
    if (!Number.isFinite(seconds) || seconds < 0) return '0:00';
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  }

  function handleKeydown(event) {
    // Do not hijack keys while typing in a form element.
    const tag = event.target?.tagName?.toLowerCase();
    if (tag === 'input' || tag === 'textarea' || tag === 'select') return;

    switch (event.key) {
      case 'Escape':
        if (!isFullscreen) {
          handleClose();
        }
        break;
      case ' ':
        event.preventDefault();
        togglePlayPause();
        break;
      case 'f':
        toggleFullscreen();
        break;
      case 'ArrowLeft':
        if (mediaElement) mediaElement.currentTime = Math.max(0, mediaElement.currentTime - 10);
        break;
      case 'ArrowRight':
        if (mediaElement) mediaElement.currentTime = Math.min(mediaElement.duration || Infinity, mediaElement.currentTime + 10);
        break;
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="player-container" class:audio-mode={isAudio}>
  {#if isAudio}
    <div class="audio-backdrop" style="background-image: url('{coverUrl}')"></div>
    <div class="audio-backdrop-shade"></div>
  {/if}

  <div class="player-header" class:audio-header={isAudio}>
    {#if isAudio}
      <div class="audio-header-label">Jellyfin Share</div>
    {:else}
      <h2>{title}</h2>
    {/if}
    <button class="close-button" on:click={handleClose} title="Close">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>

  <div class="video-wrapper" class:audio-wrapper={isAudio}>
    {#if error}
      <div class="error-overlay">
        <p>{error}</p>
        <button on:click={handleClose}>Go Back</button>
      </div>
    {/if}

    {#if isAudio}
      <div class="audio-shell">
        <div class="audio-card">
          <div class="cover-column">
            <div class="album-art">
              {#if coverUrl}
                <img src={coverUrl} alt="" />
              {:else}
                <div class="album-art-fallback" aria-hidden="true">
                  <svg viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 3v10.55A4 4 0 1 0 14 17V7h4V3h-6z"/>
                  </svg>
                </div>
              {/if}
              {#if currentIndex >= 0 && totalItems > 0}
                <div class="track-counter">{currentIndex + 1} / {totalItems}</div>
              {/if}
            </div>
          </div>

          <div class="audio-panel">
            <div class="now-playing-label">NOW PLAYING</div>
            <h1>{title}</h1>
            {#if artist}
              <div class="track-artist">{artist}</div>
            {/if}
            {#if subtitle}
              <div class="album-title">{subtitle}</div>
            {/if}

            <audio
              bind:this={mediaElement}
              autoplay
              on:loadedmetadata={handleLoadedMetadata}
              on:timeupdate={handleTimeUpdate}
              on:play={() => isPaused = false}
              on:pause={() => isPaused = true}
              on:ended={handleEnded}
            ></audio>

            <div class="timeline">
              <div class="seekbar">
                <div class="seekbar-track">
                  <div class="seekbar-fill" style="width: {progressPercent}%"></div>
                </div>
                <input
                  class="seek-input"
                  type="range"
                  min="0"
                  max={duration || 0}
                  step="0.1"
                  value={currentTime}
                  on:input={handleSeek}
                  aria-label="Seek"
                />
              </div>
              <div class="time-row">
                <span>{formatClock(currentTime)}</span>
                <span>{formatClock(duration)}</span>
              </div>
            </div>

            <div class="transport-controls">
              <button
                class="mode-button"
                class:active={shuffleEnabled}
                on:click={toggleShuffle}
                title={shuffleEnabled ? 'Shuffle on' : 'Shuffle off'}
                aria-label={shuffleEnabled ? 'Disable shuffle' : 'Enable shuffle'}
              >
                <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M16 3h5v5h-2V6.41l-4.29 4.3-1.42-1.42L17.59 5H16V3zM4 7h3.17l9.41 9.41L19 14v-1h2v5h-5v-2h1.59L6.34 8.83A1 1 0 0 0 5.63 8H4V7zm0 9h1.63a1 1 0 0 0 .71-.29l3.17-3.17 1.42 1.42-3.17 3.17A3 3 0 0 1 5.63 18H4v-2z"/>
                </svg>
              </button>

              <button
                class="transport-button secondary"
                on:click={handlePrevious}
                title="Previous track"
                disabled={!hasPrevious}
              >
                <svg viewBox="0 0 24 24" fill="currentColor">
                  <path d="M6 6h2v12H6zm3.5 6 8.5 6V6z"/>
                </svg>
              </button>

              <button class="play-pause-button" on:click={togglePlayPause} title={isPaused ? 'Play' : 'Pause'}>
                {#if isPaused}
                  <svg viewBox="0 0 24 24" fill="currentColor">
                    <path d="M8 5v14l11-7z"/>
                  </svg>
                {:else}
                  <svg viewBox="0 0 24 24" fill="currentColor">
                    <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/>
                  </svg>
                {/if}
              </button>

              <button
                class="transport-button secondary"
                on:click={handleNext}
                title="Next track"
                disabled={!hasNext}
              >
                <svg viewBox="0 0 24 24" fill="currentColor">
                  <path d="M16 6h2v12h-2zM6 18l8.5-6L6 6z"/>
                </svg>
              </button>

              <button
                class="mode-button repeat-button"
                class:active={repeatMode !== 'off'}
                on:click={toggleRepeat}
                title={repeatMode === 'one' ? 'Repeat one' : (repeatMode === 'all' ? 'Repeat all' : 'Repeat off')}
                aria-label="Change repeat mode"
              >
                <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M7 7h10V4l4 4-4 4V9H7a3 3 0 0 0-3 3v1H2v-1a5 5 0 0 1 5-5zm10 10H7v3l-4-4 4-4v3h10a3 3 0 0 0 3-3v-1h2v1a5 5 0 0 1-5 5z"/>
                </svg>
                {#if repeatMode === 'one'}
                  <span class="repeat-one-badge">1</span>
                {/if}
              </button>
            </div>

            <div class="volume-row">
              <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3A4.5 4.5 0 0 0 14 7.97v8.06A4.5 4.5 0 0 0 16.5 12zM14 3.23v2.06A7 7 0 0 1 14 18.7v2.06A9 9 0 0 0 14 3.23z"/>
              </svg>
              <input
                class="volume-input"
                type="range"
                min="0"
                max="1"
                step="0.01"
                value={volume}
                on:input={handleVolume}
                aria-label="Volume"
              />
              <span>{volumePercent}%</span>
            </div>
          </div>
        </div>
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

  {#if !isAudio}
    <div class="player-controls">
      <button on:click={toggleFullscreen} title="Toggle fullscreen (F)">
        {#if isFullscreen}
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M5 16h3v3h2v-5H5v2zm3-8H5v2h5V5H8v3zm6 11h2v-3h3v-2h-5v5zm2-11V5h-2v5h5V8h-3z"/>
          </svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z"/>
          </svg>
        {/if}
      </button>
    </div>
  {/if}
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

  .player-container.audio-mode {
    overflow: hidden;
    background: #0a0a0c;
  }

  .audio-backdrop {
    position: absolute;
    inset: -50px;
    background-position: center;
    background-size: cover;
    filter: blur(46px) saturate(1.15);
    opacity: 0.34;
    transform: scale(1.08);
  }

  .audio-backdrop-shade {
    position: absolute;
    inset: 0;
    background:
      radial-gradient(circle at 50% 42%, rgba(255,255,255,0.08), transparent 44%),
      linear-gradient(180deg, rgba(6,6,8,0.38) 0%, rgba(6,6,8,0.82) 68%, #070709 100%);
  }

  .player-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    background: linear-gradient(to bottom, rgba(0,0,0,0.8) 0%, transparent 100%);
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    z-index: 20;
  }

  .player-header.audio-header {
    padding: 1.1rem 1.35rem;
    background: transparent;
  }

  .audio-header-label {
    color: rgba(255,255,255,0.54);
    font-size: 0.78rem;
    font-weight: 650;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  .player-header h2 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0;
    color: #fff;
    text-shadow: 0 2px 4px rgba(0,0,0,0.5);
  }

  .close-button {
    width: 42px;
    height: 42px;
    border-radius: 50%;
    border: 1px solid rgba(255,255,255,0.13);
    background: rgba(20,20,24,0.44);
    backdrop-filter: blur(14px);
    color: #fff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.18s, transform 0.18s, border-color 0.18s;
  }

  .close-button:hover {
    background: rgba(255,255,255,0.16);
    border-color: rgba(255,255,255,0.24);
    transform: scale(1.04);
  }

  .close-button svg {
    width: 23px;
    height: 23px;
  }

  .video-wrapper {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    min-height: 0;
  }

  .video-wrapper.audio-wrapper {
    z-index: 5;
    padding: 72px 24px 28px;
  }

  video {
    width: 100%;
    height: 100%;
    max-height: 100vh;
    background: #000;
  }

  .audio-shell {
    width: min(920px, 94vw);
    margin: auto;
  }

  .audio-card {
    display: grid;
    grid-template-columns: minmax(240px, 350px) minmax(300px, 1fr);
    gap: clamp(28px, 5vw, 64px);
    align-items: center;
    padding: clamp(24px, 4vw, 48px);
    border: 1px solid rgba(255,255,255,0.12);
    border-radius: 30px;
    background: linear-gradient(145deg, rgba(30,30,36,0.74), rgba(11,11,14,0.62));
    backdrop-filter: blur(28px);
    box-shadow: 0 28px 80px rgba(0,0,0,0.42);
  }

  .cover-column {
    min-width: 0;
  }

  .album-art {
    position: relative;
    width: 100%;
    aspect-ratio: 1;
    overflow: hidden;
    border-radius: 22px;
    background: rgba(255,255,255,0.06);
    box-shadow: 0 24px 55px rgba(0,0,0,0.38);
  }

  .album-art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .album-art-fallback {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: rgba(255,255,255,0.68);
    background:
      radial-gradient(circle at 35% 30%, rgba(0,164,220,0.35), transparent 44%),
      rgba(255,255,255,0.06);
  }

  .album-art-fallback svg {
    width: 38%;
    height: 38%;
  }

  .track-counter {
    position: absolute;
    right: 12px;
    bottom: 12px;
    padding: 7px 10px;
    border: 1px solid rgba(255,255,255,0.16);
    border-radius: 999px;
    background: rgba(8,8,10,0.66);
    backdrop-filter: blur(12px);
    color: rgba(255,255,255,0.86);
    font-size: 0.76rem;
    font-weight: 600;
  }

  .audio-panel {
    min-width: 0;
    color: #fff;
  }

  .now-playing-label {
    margin-bottom: 10px;
    color: #00a4dc;
    font-size: 0.76rem;
    font-weight: 750;
    letter-spacing: 0.13em;
  }

  .audio-panel h1 {
    margin: 0;
    color: #fff;
    font-size: clamp(1.55rem, 3.1vw, 2.55rem);
    font-weight: 720;
    line-height: 1.12;
    letter-spacing: -0.02em;
    overflow-wrap: anywhere;
  }

  .track-artist {
    margin-top: 0.4rem;
    font-size: 1.05rem;
    font-weight: 500;
    color: rgba(255, 255, 255, 0.82);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .album-title {
    margin-top: 9px;
    color: rgba(255,255,255,0.58);
    font-size: 1rem;
    line-height: 1.4;
  }

  audio {
    display: none;
  }

  .timeline {
    margin-top: clamp(28px, 5vh, 50px);
  }

  .seekbar {
    position: relative;
    height: 22px;
    display: flex;
    align-items: center;
  }

  .seekbar-track {
    position: absolute;
    left: 0;
    right: 0;
    height: 5px;
    overflow: hidden;
    border-radius: 999px;
    background: rgba(255,255,255,0.16);
  }

  .seekbar-fill {
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, #00a4dc, #43d6ff);
    box-shadow: 0 0 16px rgba(0,164,220,0.45);
  }

  .seek-input {
    position: absolute;
    inset: 0;
    width: 100%;
    margin: 0;
    opacity: 0;
    cursor: pointer;
  }

  .time-row {
    display: flex;
    justify-content: space-between;
    margin-top: 1px;
    color: rgba(255,255,255,0.48);
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
  }

  .transport-controls {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 16px;
    margin-top: 26px;
  }

  .transport-controls button {
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    color: #fff;
    cursor: pointer;
    transition: transform 0.16s, background 0.16s, opacity 0.16s;
  }

  .transport-controls button:hover:not(:disabled) {
    transform: scale(1.06);
  }

  .transport-controls button:active:not(:disabled) {
    transform: scale(0.96);
  }

  .transport-button {
    width: 48px;
    height: 48px;
    border-radius: 50%;
  }

  .transport-button.secondary {
    background: rgba(255,255,255,0.09);
  }

  .transport-button.secondary:hover:not(:disabled) {
    background: rgba(255,255,255,0.16);
  }

  .mode-button {
    position: relative;
    width: 40px;
    height: 40px;
    border-radius: 50%;
    background: transparent;
    color: rgba(255,255,255,0.48) !important;
  }

  .mode-button:hover {
    background: rgba(255,255,255,0.08);
    color: rgba(255,255,255,0.84) !important;
  }

  .mode-button.active {
    color: #63bfff !important;
    background: rgba(0,164,220,0.12);
  }

  .mode-button svg {
    width: 21px;
    height: 21px;
  }

  .repeat-one-badge {
    position: absolute;
    right: 5px;
    bottom: 4px;
    min-width: 13px;
    height: 13px;
    padding: 0 2px;
    border-radius: 7px;
    background: #63bfff;
    color: #061018;
    font-size: 9px;
    line-height: 13px;
    font-weight: 800;
    text-align: center;
  }

  .transport-controls button:disabled {
    opacity: 0.24;
    cursor: default;
  }

  .play-pause-button {
    width: 70px;
    height: 70px;
    border-radius: 50%;
    background: #fff;
    color: #08080a !important;
    box-shadow: 0 12px 32px rgba(0,0,0,0.28);
  }

  .transport-button svg {
    width: 23px;
    height: 23px;
  }

  .play-pause-button svg {
    width: 30px;
    height: 30px;
  }

  .volume-row {
    display: grid;
    grid-template-columns: 18px 1fr 38px;
    align-items: center;
    gap: 10px;
    margin-top: 30px;
    color: rgba(255,255,255,0.48);
    font-size: 0.76rem;
    font-variant-numeric: tabular-nums;
  }

  .volume-row > svg {
    width: 18px;
    height: 18px;
  }

  .volume-input {
    width: 100%;
    height: 4px;
    margin: 0;
    accent-color: #00a4dc;
    cursor: pointer;
  }

  .error-overlay {
    position: absolute;
    inset: 0;
    z-index: 100;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background: rgba(0,0,0,0.9);
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
    background: rgba(255,255,255,0.1);
    color: #fff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.2s;
  }

  .player-controls button:hover {
    background: rgba(255,255,255,0.2);
  }

  .player-controls svg {
    width: 24px;
    height: 24px;
  }

  @media (max-width: 760px) {
    .video-wrapper.audio-wrapper {
      padding: 70px 14px 18px;
      overflow-y: auto;
    }

    .audio-shell {
      width: min(440px, 100%);
    }

    .audio-card {
      grid-template-columns: 1fr;
      gap: 22px;
      padding: 20px;
      border-radius: 24px;
    }

    .cover-column {
      width: min(270px, 72vw);
      margin: 0 auto;
    }

    .audio-panel {
      text-align: center;
    }

    .audio-panel h1 {
      font-size: clamp(1.35rem, 6vw, 1.9rem);
    }

    .album-title {
      font-size: 0.92rem;
    }

    .timeline {
      margin-top: 24px;
      text-align: left;
    }

    .volume-row {
      max-width: 300px;
      margin-left: auto;
      margin-right: auto;
    }

    .transport-controls {
      gap: 10px;
    }

    .mode-button {
      width: 36px;
      height: 36px;
    }

    .transport-button {
      width: 44px;
      height: 44px;
    }

    .play-pause-button {
      width: 64px;
      height: 64px;
    }
  }

  @media (max-height: 620px) and (min-width: 761px) {
    .video-wrapper.audio-wrapper {
      padding-top: 62px;
      padding-bottom: 16px;
    }

    .audio-card {
      padding: 24px;
      gap: 30px;
    }

    .timeline {
      margin-top: 20px;
    }

    .transport-controls {
      margin-top: 16px;
    }

    .volume-row {
      margin-top: 18px;
    }
  }
</style>
