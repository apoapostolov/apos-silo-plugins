// Silo Prerolls Web Client Patch
// Inject this script into Silo's index.html to enable preroll playback.
//
// Usage: Add this script tag to the Silo web client's index.html:
// <script src="/plugins/prerolls/web/prerolls.js"></script>
//
// Or inject via a reverse proxy that adds the script tag.

(function() {
  'use strict';

  const PREROLLS_API = '/api/v2/plugins/prerolls';

  // Configuration
  let config = null;
  let skipButton = null;
  let prerollPlaying = false;

  // Fetch plugin config
  async function loadConfig() {
    try {
      const resp = await fetch(`${PREROLLS_API}/config`);
      if (resp.ok) {
        config = await resp.json();
      }
    } catch (e) {
      console.warn('[Prerolls] Failed to load config:', e);
    }
  }

  // Discover preroll for an item
  async function discoverPreroll(itemId, userId) {
    try {
      const url = `${PREROLLS_API}/discover?item_id=${encodeURIComponent(itemId)}`;
      const resp = await fetch(url);
      if (resp.ok) {
        return await resp.json();
      }
    } catch (e) {
      console.warn('[Prerolls] Failed to discover:', e);
    }
    return null;
  }

  // Record skip
  async function recordSkip(prerollId, itemId, seconds) {
    try {
      await fetch(`${PREROLLS_API}/skip`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ preroll_id: prerollId, item_id: itemId, seconds: seconds })
      });
    } catch (e) {
      console.warn('[Prerolls] Failed to record skip:', e);
    }
  }

  // Create skip button
  function createSkipButton(delay) {
    const btn = document.createElement('button');
    btn.className = 'preroll-skip-btn';
    btn.textContent = `Skip in ${delay}s`;
    btn.style.cssText = `
      position: fixed;
      bottom: 40px;
      right: 40px;
      z-index: 9999;
      padding: 10px 20px;
      background: rgba(0,0,0,0.7);
      color: white;
      border: 1px solid rgba(255,255,255,0.3);
      border-radius: 4px;
      cursor: pointer;
      font-size: 14px;
      font-family: sans-serif;
    `;
    return btn;
  }

  // Play preroll before main content
  async function playPreroll(itemId, videoElement, onDone) {
    if (!config || !config.enabled) {
      onDone();
      return;
    }

    const preroll = await discoverPreroll(itemId);
    if (!preroll) {
      onDone();
      return;
    }

    prerollPlaying = true;
    const startTime = Date.now();

    // Create skip button
    let skipDelay = config.skip_delay_seconds || 5;
    skipButton = createSkipButton(skipDelay);
    document.body.appendChild(skipButton);

    // Set up video
    const originalSrc = videoElement.src;
    videoElement.src = preroll.path;
    videoElement.play();

    // Skip button countdown
    const countdown = setInterval(() => {
      skipDelay--;
      if (skipDelay <= 0) {
        skipButton.textContent = 'Skip';
        skipButton.onclick = () => {
          clearInterval(countdown);
          finishPreroll(videoElement, originalSrc, preroll, itemId, startTime, true);
        };
      } else {
        skipButton.textContent = `Skip in ${skipDelay}s`;
      }
    }, 1000);

    // When preroll ends, play main content
    videoElement.onended = () => {
      clearInterval(countdown);
      finishPreroll(videoElement, originalSrc, preroll, itemId, startTime, false);
    };

    // Handle errors
    videoElement.onerror = () => {
      clearInterval(countdown);
      if (skipButton && skipButton.parentNode) {
        skipButton.parentNode.removeChild(skipButton);
      }
      skipButton = null;
      prerollPlaying = false;
      onDone();
    };
  }

  function finishPreroll(videoElement, originalSrc, preroll, itemId, startTime, skipped) {
    if (skipButton && skipButton.parentNode) {
      skipButton.parentNode.removeChild(skipButton);
    }
    skipButton = null;
    prerollPlaying = false;

    if (skipped) {
      const elapsed = (Date.now() - startTime) / 1000;
      recordSkip(preroll.id, itemId, elapsed);
    }

    // Play main content
    videoElement.src = originalSrc;
    videoElement.play();
  }

  // Hook into Silo's playback (placeholder - needs Silo-specific integration)
  function init() {
    loadConfig();
    console.log('[Prerolls] Silo Prerolls web client patch loaded');
  }

  // Initialize when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
