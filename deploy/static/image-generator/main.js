(() => {
  const $ = (sel, ctx = document) => ctx.querySelector(sel);
  const $$ = (sel, ctx = document) => Array.from(ctx.querySelectorAll(sel));

  function cleanText(value) {
    return String(value || '')
      .replace(/[\u200B-\u200D\uFEFF]/g, '')
      .replace(/`/g, '')
      .trim();
  }

  function cleanURL(value) {
    const raw = cleanText(value);
    try {
      return new URL(raw).origin;
    } catch {
      return raw.replace(/\s/g, '').replace(/\/+$/, '');
    }
  }


  function escapeHTML(value) {
    return String(value || '').replace(/[&<>'"]/g, ch => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      "'": '&#39;',
      '"': '&quot;'
    }[ch]));
  }

  const _urlParams = new URLSearchParams(window.location.search);
  const iframeState = {
    token: cleanText(_urlParams.get('token')),
    srcHost: cleanURL(_urlParams.get('src_host')),
    userId: cleanText(_urlParams.get('user_id')),
    isEmbedded: cleanText(_urlParams.get('ui_mode')) === 'embedded'
  };
  try {
    if (!iframeState.token) iframeState.token = cleanText(localStorage.getItem('auth_token'));
  } catch (e) { /* localStorage unavailable */ }
  if (!iframeState.srcHost && typeof window !== 'undefined') {
    iframeState.srcHost = window.location.origin;
  }
  console.log('[生图调试] iframe参数:', {
    hasToken: !!iframeState.token,
    tokenLen: iframeState.token.length,
    srcHost: iframeState.srcHost,
    userId: iframeState.userId,
    isEmbedded: iframeState.isEmbedded
  });

  (function initDarkMode() {
    const root = document.documentElement;

    function applyTheme(isDark) {
      root.setAttribute('data-theme', isDark ? 'dark' : 'light');
      console.log('[深色模式]', isDark ? '深色' : '浅色');
    }

    function getSystemDark() {
      try {
        return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
      } catch (e) {
        return false;
      }
    }

    function getParentDark() {
      try {
        if (window.parent && window.parent !== window) {
          const parentDoc = window.parent.document;
          const parentTheme = parentDoc.documentElement.getAttribute('data-theme');
          if (parentTheme) return parentTheme === 'dark';
          const parentCS = window.parent.getComputedStyle(parentDoc.documentElement);
          if (parentCS.colorScheme && parentCS.colorScheme.includes('dark')) return true;
          const parentBg = parentCS.backgroundColor;
          if (parentBg) {
            const match = parentBg.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
            if (match) {
              const brightness = (parseInt(match[1]) * 299 + parseInt(match[2]) * 587 + parseInt(match[3]) * 114) / 1000;
              return brightness < 128;
            }
          }
        }
      } catch (e) {
        console.log('[深色模式] 无法访问父级窗口:', e.message);
      }
      return null;
    }

    let currentDark = false;

    function syncTheme() {
      const parentDark = getParentDark();
      const isDark = parentDark !== null ? parentDark : getSystemDark();
      if (isDark !== currentDark) {
        currentDark = isDark;
        applyTheme(isDark);
      }
    }

    syncTheme();

    try {
      if (window.parent && window.parent !== window) {
        const parentDoc = window.parent.document;
        const observer = new MutationObserver(function() {
          syncTheme();
        });
        observer.observe(parentDoc.documentElement, {
          attributes: true,
          attributeFilter: ['data-theme', 'class', 'style']
        });
        console.log('[深色模式] 已监听父级窗口变化');
      }
    } catch (e) {
      console.log('[深色模式] 无法监听父级窗口:', e.message);
    }

    try {
      if (window.matchMedia) {
        const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
        const handler = function() {
          if (getParentDark() === null) {
            syncTheme();
          }
        };
        if (mediaQuery.addEventListener) {
          mediaQuery.addEventListener('change', handler);
        } else if (mediaQuery.addListener) {
          mediaQuery.addListener(handler);
        }
        console.log('[深色模式] 已监听系统主题变化');
      }
    } catch (e) {
      console.log('[深色模式] 无法监听系统主题:', e.message);
    }

    window.addEventListener('focus', syncTheme);
    setInterval(syncTheme, 2000);
  })();

  async function callSub2API(path, options = {}) {
    const requestOrigin = options.origin || (typeof window !== 'undefined' ? window.location.origin : iframeState.srcHost);
    if (!requestOrigin) return null;

    const controller = new AbortController();
    const timeoutMs = options.timeoutMs || 8000;
    const timeoutId = setTimeout(() => controller.abort(), timeoutMs);
    const url = new URL(path, requestOrigin + '/').toString();

    try {
      const headers = { 'Accept': 'application/json' };
      if (iframeState.token) {
        headers.Authorization = 'Bearer ' + iframeState.token;
      }
      const resp = await fetch(url, {
        headers,
        mode: 'cors',
        credentials: 'omit',
        cache: 'no-store',
        signal: controller.signal
      });
      if (!resp.ok) {
        const text = await resp.text();
        throw new Error('API ' + resp.status + ': ' + text);
      }
      return resp.json();
    } finally {
      clearTimeout(timeoutId);
    }
  }

  async function callSub2APIJSON(path, payload, options = {}) {
    if (!iframeState.token) throw new Error('请先登录后再发布');
    const requestOrigin = options.origin || (typeof window !== 'undefined' ? window.location.origin : iframeState.srcHost);
    if (!requestOrigin) throw new Error('无法确定服务地址');

    const controller = new AbortController();
    const timeoutMs = options.timeoutMs || 15000;
    const timeoutId = setTimeout(() => controller.abort(), timeoutMs);
    const url = new URL(path, requestOrigin + '/').toString();

    try {
      const resp = await fetch(url, {
        method: options.method || 'POST',
        headers: {
          'Authorization': 'Bearer ' + iframeState.token,
          'Accept': 'application/json',
          'Content-Type': 'application/json'
        },
        mode: 'cors',
        credentials: 'omit',
        cache: 'no-store',
        signal: controller.signal,
        body: JSON.stringify(payload || {})
      });
      const text = await resp.text();
      let json = null;
      try { json = text ? JSON.parse(text) : null; } catch {}
      if (!resp.ok) {
        throw new Error(json?.message || json?.error?.message || text || ('API ' + resp.status));
      }
      return json;
    } finally {
      clearTimeout(timeoutId);
    }
  }

  function showToast(message, type = 'info') {
    let container = $('#toast-container');
    if (!container) {
      container = document.createElement('div');
      container.id = 'toast-container';
      document.body.appendChild(container);
    }
    const toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    toast.textContent = message;
    container.appendChild(toast);
    requestAnimationFrame(() => toast.classList.add('toast-visible'));
    setTimeout(() => {
      toast.classList.remove('toast-visible');
      toast.addEventListener('transitionend', () => toast.remove(), { once: true });
      setTimeout(() => toast.remove(), 400);
    }, 3000);
  }

  function showConfirm(title, message) {
    return new Promise(function(resolve) {
      var overlay = document.createElement('div');
      overlay.className = 'confirm-overlay';
      overlay.innerHTML = '<div class="confirm-backdrop"></div><div class="confirm-dialog"><div class="confirm-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"/></svg></div><h3 class="confirm-title"></h3><p class="confirm-message"></p><div class="confirm-actions"><button type="button" class="confirm-btn confirm-btn-cancel">取消</button><button type="button" class="confirm-btn confirm-btn-confirm">确认清空</button></div></div>';
      overlay.querySelector('.confirm-title').textContent = title;
      overlay.querySelector('.confirm-message').textContent = message;
      document.body.appendChild(overlay);
      requestAnimationFrame(function() { overlay.classList.add('confirm-active'); });

      function close(result) {
        overlay.classList.remove('confirm-active');
        setTimeout(function() { overlay.remove(); }, 250);
        resolve(result);
      }

      overlay.querySelector('.confirm-backdrop').addEventListener('click', function() { close(false); });
      overlay.querySelector('.confirm-btn-cancel').addEventListener('click', function() { close(false); });
      overlay.querySelector('.confirm-btn-confirm').addEventListener('click', function() { close(true); });
      document.addEventListener('keydown', function handler(e) {
        if (e.key === 'Escape') {
          document.removeEventListener('keydown', handler);
          close(false);
        }
      });
    });
  }

  (() => {
    const modes = ['text', 'image', 'history', 'gallery'];
    const tabs = $$('.image-tabs .image-tab[data-mode]');
    const panels = $$('[data-panel]');
    if (!tabs.length || !panels.length) return;

    function setMode(mode, updateHash = false) {
      const next = modes.includes(mode) ? mode : modes[0];
      for (const tab of tabs) {
        const active = tab.dataset.mode === next;
        tab.classList.toggle('image-tab-active', active);
        tab.setAttribute('aria-selected', active ? 'true' : 'false');
        tab.tabIndex = active ? 0 : -1;
      }
      for (const panel of panels) {
        const active = panel.dataset.panel === next;
        panel.hidden = !active;
        panel.setAttribute('aria-hidden', active ? 'false' : 'true');
      }
      if (updateHash) {
        history.replaceState(null, '', '#' + next);
      }
    }

    tabs.forEach(tab => {
      tab.addEventListener('click', () => setMode(tab.dataset.mode, true));
    });

    const tabList = $('.image-tabs');
    if (tabList) {
      tabList.addEventListener('keydown', e => {
        const current = tabs.findIndex(t => t.classList.contains('image-tab-active'));
        let next = -1;
        if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
          e.preventDefault();
          next = (current + 1) % tabs.length;
        } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
          e.preventDefault();
          next = (current - 1 + tabs.length) % tabs.length;
        } else if (e.key === 'Home') {
          e.preventDefault();
          next = 0;
        } else if (e.key === 'End') {
          e.preventDefault();
          next = tabs.length - 1;
        }
        if (next >= 0) {
          tabs[next].focus();
          setMode(tabs[next].dataset.mode, true);
        }
      });
    }

    const fromHash = location.hash.replace('#', '');
    const initial = modes.includes(fromHash) ? fromHash : (tabs.find(t => t.classList.contains('image-tab-active'))?.dataset.mode || modes[0]);
    setMode(initial, false);
  })();

  (() => {
    const panels = $$('.advanced-panel');
    if (!panels.length) return;

    const motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
    const duration = 300;

    function setExpanded(details, expanded) {
      const summary = details.querySelector('summary');
      if (summary) summary.setAttribute('aria-expanded', expanded ? 'true' : 'false');
    }

    function finishAnimation(body, callback) {
      let done = false;
      const cleanup = () => {
        if (done) return;
        done = true;
        body.removeEventListener('transitionend', onEnd);
        callback();
      };
      const onEnd = event => {
        if (event.target === body && event.propertyName === 'height') cleanup();
      };
      body.addEventListener('transitionend', onEnd);
      window.setTimeout(cleanup, duration + 60);
    }

    function openPanel(details, body) {
      details.open = true;
      setExpanded(details, true);
      body.style.paddingBottom = '0px';
      if (motionQuery.matches) {
        body.style.height = 'auto';
        body.style.opacity = '1';
        body.style.paddingBottom = '';
        return;
      }
      body.style.height = '0px';
      body.style.opacity = '0';
      body.style.transform = 'translateY(-6px)';
      body.dataset.animating = 'true';
      requestAnimationFrame(() => {
        body.style.paddingBottom = '';
        body.style.height = body.scrollHeight + 'px';
        body.style.opacity = '1';
        body.style.transform = 'translateY(0)';
      });
      finishAnimation(body, () => {
        body.dataset.animating = 'false';
        body.style.height = 'auto';
        body.style.paddingBottom = '';
        body.style.transform = '';
      });
    }

    function closePanel(details, body) {
      setExpanded(details, false);
      body.style.paddingBottom = window.getComputedStyle(body).paddingBottom;
      if (motionQuery.matches) {
        body.style.height = '0px';
        body.style.opacity = '0';
        details.open = false;
        body.style.paddingBottom = '';
        return;
      }
      body.style.height = body.scrollHeight + 'px';
      body.style.opacity = '1';
      body.dataset.animating = 'true';
      requestAnimationFrame(() => {
        body.style.height = '0px';
        body.style.opacity = '0';
        body.style.paddingBottom = '0px';
        body.style.transform = 'translateY(-6px)';
      });
      finishAnimation(body, () => {
        body.dataset.animating = 'false';
        details.open = false;
        body.style.paddingBottom = '';
        body.style.transform = '';
      });
    }

    panels.forEach(details => {
      const summary = details.querySelector('summary');
      const body = details.querySelector('.advanced-body');
      if (!summary || !body) return;
      details.open = false;
      body.style.height = '0px';
      body.style.opacity = '0';
      summary.setAttribute('aria-expanded', 'false');
      summary.addEventListener('click', event => {
        event.preventDefault();
        if (body.dataset.animating === 'true') return;
        if (details.open) {
          closePanel(details, body);
        } else {
          openPanel(details, body);
        }
      });
    });
  })();

  // ============================================================
  // SizeIntent 尺寸意图自动降级模块
  // ============================================================

  const SIZE_MATRIX = {
    '1:1': {
      '1k': [1024, 1024],
      '2k': [2048, 2048],
      '4k': [4096, 4096]
    },
    '4:3': {
      '1k': [1365, 1024],
      '2k': [2730, 2048],
      '4k': [5461, 4096]
    },
    '3:4': {
      '1k': [1024, 1365],
      '2k': [2048, 2730],
      '4k': [4096, 5461]
    },
    '16:9': {
      '1k': [1536, 864],
      '2k': [2048, 1152],
      '4k': [3840, 2160]
    },
    '9:16': {
      '1k': [864, 1536],
      '2k': [1152, 2048],
      '4k': [2160, 3840]
    }
  };

  const ASPECT_PROMPT_MAP = {
    '1:1': 'square composition',
    '4:3': 'landscape composition, 4:3 aspect ratio',
    '3:4': 'portrait composition, 3:4 aspect ratio',
    '16:9': 'cinematic wide composition, 16:9 aspect ratio',
    '9:16': 'vertical mobile composition, 9:16 aspect ratio'
  };

  const RESOLUTION_PROMPT_MAP = {
    '1k': 'high detail',
    '2k': 'ultra detailed, high resolution',
    '4k': 'extremely detailed, 4k quality, ultra high resolution'
  };

  function getGPTModelCapabilities(modelID) {
    const normalized = String(modelID || '').toLowerCase();
    
    if (normalized.startsWith('gpt-image') || normalized.startsWith('chatgpt-image')) {
      return {
        supportsSize: true,
        supportedSizes: ['1024x1024', '1536x1024', '1024x1536'],
        maxResolution: 1536
      };
    }
    
    if (normalized.startsWith('dall-e-3')) {
      return {
        supportsSize: true,
        supportedSizes: ['1024x1024', '1792x1024', '1024x1792'],
        maxResolution: 1792
      };
    }
    
    if (normalized.startsWith('dall-e-2')) {
      return {
        supportsSize: true,
        supportedSizes: ['256x256', '512x512', '1024x1024'],
        maxResolution: 1024
      };
    }
    
    return {
      supportsSize: true,
      supportedSizes: ['1024x1024', '1536x1024', '1024x1536'],
      maxResolution: 1536
    };
  }

  function buildSizeIntent(ratio, resolutionTier) {
    const isAutoRatio = (ratio === '自动生成');
    const isAutoTier = (resolutionTier === 'auto');
    
    if (isAutoRatio && isAutoTier) {
      return {
        aspect: 'auto',
        resolution: 'auto',
        width: null,
        height: null,
        size: 'auto',
        aspectRatio: null,
        promptEnhancement: ''
      };
    }
    
    const aspectKey = isAutoRatio ? '1:1' : ratio.replace(/ 正方形| 横版| 竖版/g, '');
    const resolutionKey = isAutoTier ? '1k' : resolutionTier.toLowerCase();
    
    const sizeEntry = SIZE_MATRIX[aspectKey]?.[resolutionKey];
    const width = sizeEntry ? sizeEntry[0] : 1024;
    const height = sizeEntry ? sizeEntry[1] : 1024;
    
    const aspectDesc = ASPECT_PROMPT_MAP[aspectKey] || '';
    const resolutionDesc = RESOLUTION_PROMPT_MAP[resolutionKey] || '';
    const promptEnhancement = [aspectDesc, resolutionDesc].filter(Boolean).join(', ');
    
    return {
      aspect: aspectKey,
      resolution: resolutionKey,
      width,
      height,
      size: `${width}x${height}`,
      aspectRatio: `${width}:${height}`,
      promptEnhancement
    };
  }

  function applySizeIntentToTool(tool, sizeIntent, modelID) {
    const capabilities = getGPTModelCapabilities(modelID);
    
    console.log('[生图调试] ===== 尺寸应用 =====');
    console.log('[生图调试] 模型ID:', modelID);
    console.log('[生图调试] 模型能力:', JSON.stringify(capabilities, null, 2));
    console.log('[生图调试] 期望尺寸:', sizeIntent.size);
    
    if (sizeIntent.size === 'auto') {
      tool.size = 'auto';
      console.log('[生图调试] 结果: 自动模式，使用auto');
      return tool;
    }
    
    if (capabilities.supportedSizes.includes(sizeIntent.size)) {
      tool.size = sizeIntent.size;
      console.log('[生图调试] 结果: 模型直接支持，使用', tool.size);
      return tool;
    }
    
    console.log('[生图调试] 模型不直接支持', sizeIntent.size, '，开始降级...');
    
    const scale = Math.min(
      capabilities.maxResolution / sizeIntent.width,
      capabilities.maxResolution / sizeIntent.height,
      1
    );
    const scaledWidth = Math.floor(sizeIntent.width * scale);
    const scaledHeight = Math.floor(sizeIntent.height * scale);
    
    const roundTo64 = (n) => Math.round(n / 64) * 64;
    tool.size = `${roundTo64(scaledWidth)}x${roundTo64(scaledHeight)}`;
    
    console.log('[生图调试] 缩放比例:', scale.toFixed(4));
    console.log('[生图调试] 缩放后:', scaledWidth + 'x' + scaledHeight);
    console.log('[生图调试] 对齐64后:', tool.size);
    console.log('[生图调试] 最终降级: ' + sizeIntent.size + ' -> ' + tool.size + ' (模型最大' + capabilities.maxResolution + ')');
    return tool;
  }

  function enhancePromptWithSizeIntent(prompt, sizeIntent) {
    if (!sizeIntent.promptEnhancement) return prompt;
    
    const lowerPrompt = prompt.toLowerCase();
    const skipKeywords = ['aspect ratio', 'resolution', 'composition', 'detail', '4k', '2k', '1k'];
    const hasExistingSizeHint = skipKeywords.some(kw => lowerPrompt.includes(kw));
    
    if (hasExistingSizeHint) return prompt;
    
    return `${prompt}, ${sizeIntent.promptEnhancement}`;
  }

  console.log('[生图调试] SizeIntent模块已加载');

  const SELECT_OPTIONS = {
    'api-key': [
      { value: '', label: '正在加载你的 API 密钥...' }
    ],
      'model': [
        { value: 'gpt-image-2', label: 'gpt-image-2' },
        { value: 'gpt-image-1.5', label: 'gpt-image-1.5' },
        { value: 'gpt-image-1', label: 'gpt-image-1' }
      ],
      'quality': [
        { value: 'auto', label: '自动' },
        { value: 'low', label: '低' },
        { value: 'medium', label: '中' },
        { value: 'high', label: '高' }
      ],
      'background': [
        { value: 'auto', label: '自动' },
        { value: 'transparent', label: '透明' },
        { value: 'opaque', label: '不透明' }
      ],
      'format': [
        { value: 'png', label: 'PNG' },
        { value: 'webp', label: 'WebP' },
        { value: 'jpeg', label: 'JPEG' }
      ]
    };

  (() => {
    let openDropdown = null;
    const LAST_API_KEY_STORAGE_PREFIX = 'peterai_image_generator_last_api_key_v1';

    function lastApiKeyStorageKey() {
      const scope = [
        cleanText(iframeState.srcHost || window.location.origin || 'local'),
        cleanText(iframeState.userId || 'anonymous')
      ].join(':');
      return LAST_API_KEY_STORAGE_PREFIX + ':' + scope;
    }

    function readLastApiKeyPreference() {
      try {
        const raw = localStorage.getItem(lastApiKeyStorageKey());
        if (!raw) return null;
        const parsed = JSON.parse(raw);
        if (!parsed || typeof parsed !== 'object') return null;
        return {
          id: cleanText(parsed.id),
          key: cleanText(parsed.key),
          label: cleanText(parsed.label)
        };
      } catch (e) {
        return null;
      }
    }

    function saveLastApiKeyPreference(option) {
      if (!option || (!option.id && !option.value)) return;
      try {
        localStorage.setItem(lastApiKeyStorageKey(), JSON.stringify({
          id: cleanText(option.id),
          key: option.id ? '' : cleanText(option.value),
          label: cleanText(option.label),
          saved_at: Date.now()
        }));
      } catch (e) {
        console.warn('[生图调试] 保存上次API密钥失败:', e);
      }
    }

    function isApiKeyField(field) {
      const label = field?.querySelector('.input-label');
      const text = label?.textContent?.trim() || '';
      return text.includes('API') || text.includes('密钥');
    }

    function setTriggerSelection(trigger, option) {
      if (!trigger || !option) return;
      trigger.dataset.selectValue = option.value || '';
      trigger.dataset.selectLabel = option.label || '';
      trigger.dataset.selectId = option.id || '';
      const valueNode = trigger.querySelector('.select-value');
      if (valueNode) valueNode.textContent = option.label || '请选择';
    }

    function clearApiKeySelectionLabel(text) {
      $$(".select-trigger").forEach(trigger => {
        const field = trigger.closest('.image-field');
        if (!isApiKeyField(field)) return;
        trigger.dataset.selectValue = '';
        trigger.dataset.selectLabel = text || '';
        trigger.dataset.selectId = '';
        const valueNode = trigger.querySelector('.select-value');
        if (valueNode) valueNode.textContent = text;
      });
    }

    function syncApiKeySelection(option) {
      $$(".select-trigger").forEach(trigger => {
        const field = trigger.closest('.image-field');
        if (!isApiKeyField(field)) return;
        setTriggerSelection(trigger, option);
      });
    }

    function pickPreferredApiKey(options) {
      const pref = readLastApiKeyPreference();
      if (!pref) return null;
      return options.find(option => pref.id && option.id === pref.id)
        || options.find(option => pref.key && option.value === pref.key)
        || options.find(option => pref.label && option.label === pref.label)
        || null;
    }

    function closeDropdown() {
      if (openDropdown) {
        openDropdown.wrapper.classList.remove('select-open');
        if (openDropdown.menu) openDropdown.menu.remove();
        openDropdown = null;
      }
    }

    function positionDropdown(trigger, menu) {
      const rect = trigger.getBoundingClientRect();
      const gap = 4;
      const below = window.innerHeight - rect.bottom - gap;
      const above = rect.top - gap;
      const maxHeight = Math.max(160, Math.min(320, Math.max(below, above) - 8));
      const openUp = below < 180 && above > below;
      menu.style.left = rect.left + 'px';
      menu.style.width = rect.width + 'px';
      menu.style.maxHeight = maxHeight + 'px';
      if (openUp) {
        menu.style.top = 'auto';
        menu.style.bottom = (window.innerHeight - rect.top + gap) + 'px';
      } else {
        menu.style.top = (rect.bottom + gap) + 'px';
        menu.style.bottom = 'auto';
      }
    }

    document.addEventListener('click', e => {
      if (openDropdown && !openDropdown.wrapper.contains(e.target) && !openDropdown.menu.contains(e.target)) {
        closeDropdown();
      }
    });

    window.addEventListener('resize', closeDropdown);
    window.addEventListener('scroll', e => {
      if (openDropdown?.menu?.contains(e.target)) return;
      closeDropdown();
    }, true);

    document.addEventListener('keydown', e => {
      if (e.key === 'Escape') closeDropdown();
    });

    function createSelect(field, optionsKey) {
      const trigger = field.querySelector('.select-trigger');
      if (!trigger) return;
      const wrapper = trigger.closest('.relative') || trigger.parentElement;
      trigger.dataset.selectKey = optionsKey;
      if (!trigger.dataset.selectValue) {
        const initial = (SELECT_OPTIONS[optionsKey] || [])[0];
        if (initial) setTriggerSelection(trigger, initial);
      }

      trigger.addEventListener('click', e => {
        e.stopPropagation();
        if (openDropdown && openDropdown.wrapper === wrapper) {
          closeDropdown();
          return;
        }
        closeDropdown();

        const menu = document.createElement('div');
        menu.className = 'select-menu';
        const options = SELECT_OPTIONS[optionsKey] || [];
        const currentValue = trigger.dataset.selectValue || '';
        options.forEach(opt => {
          const item = document.createElement('button');
          item.type = 'button';
          item.className = 'select-option' + (opt.value === currentValue ? ' select-option-active' : '');
          item.textContent = opt.label;
          item.addEventListener('click', ev => {
            ev.stopPropagation();
            if (optionsKey === 'api-key') {
              syncApiKeySelection(opt);
              saveLastApiKeyPreference(opt);
            } else {
              setTriggerSelection(trigger, opt);
            }
            closeDropdown();
            updateCost();
          });
          menu.appendChild(item);
        });

        wrapper.classList.add('select-open');
        document.body.appendChild(menu);
        positionDropdown(trigger, menu);
        openDropdown = { wrapper, menu };

        const firstItem = menu.querySelector('.select-option');
        if (firstItem) firstItem.focus();
      });

      trigger.addEventListener('keydown', e => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          trigger.click();
        }
      });
    }

    function initPanelSelects(panel) {
      if (!panel) return;
      const fields = $$('.image-field', panel);
      fields.forEach(field => {
        const trigger = field.querySelector('.select-trigger');
        if (!trigger) return;
        const label = field.querySelector('.input-label');
        if (!label) return;
        const labelText = label.textContent.trim();
        let key = '';
        if (labelText.includes('API') || labelText.includes('密钥')) key = 'api-key';
        else if (labelText.includes('模型')) key = 'model';
        else if (labelText.includes('质量')) key = 'quality';
        else if (labelText.includes('背景')) key = 'background';
        else if (labelText.includes('输出格式')) key = 'format';
        if (key) createSelect(field, key);
      });
    }

    function setApiKeySelectLabel(text) {
      clearApiKeySelectionLabel(text);
    }

    async function fetchAndPopulateApiKeys() {
      if (!iframeState.token) {
        setApiKeySelectLabel('请先登录 sub2api 后再进入此页面');
        return;
      }

      try {
        let resp = await callSub2API('/api/v1/keys?page=1&page_size=200', { timeoutMs: 5000 }).catch((e) => {
          console.warn('[生图调试] 同源获取API密钥请求失败:', e);
          return null;
        });
        if (!resp && iframeState.srcHost && iframeState.srcHost !== window.location.origin) {
          resp = await callSub2API('/api/v1/keys?page=1&page_size=200', { origin: iframeState.srcHost, timeoutMs: 5000 }).catch((e) => {
            console.warn('[生图调试] src_host获取API密钥请求失败:', e);
            return null;
          });
        }
        const payload = resp?.data || resp || {};
        const items = Array.isArray(payload) ? payload : (payload.items || payload.data?.items || []);
        if (!Array.isArray(items) || items.length === 0) {
          setApiKeySelectLabel('暂无 API 密钥，请先在 sub2api 创建');
          return;
        }

        const apiKeys = items
          .map(item => ({
            id: item.id == null ? '' : cleanText(item.id),
            value: cleanText(item.key),
            label: cleanText(item.name || item.key || '未命名密钥')
          }))
          .filter(item => item.value);
        console.log('[生图调试] 获取到API密钥数量:', apiKeys.length, apiKeys.map(k => ({ label: k.label, valueLen: k.value.length })));

        if (apiKeys.length > 0) {
          SELECT_OPTIONS['api-key'] = apiKeys;
        }

        const preferred = pickPreferredApiKey(SELECT_OPTIONS['api-key']) || SELECT_OPTIONS['api-key'][0];
        if (preferred) {
          syncApiKeySelection(preferred);
          console.log('[生图调试] 已选择API密钥:', { label: preferred.label, restored: preferred !== SELECT_OPTIONS['api-key'][0] });
        } else {
          setApiKeySelectLabel('请选择 API 密钥');
        }
      } catch (e) {
        console.warn('[生图调试] 获取API密钥失败:', e);
        setApiKeySelectLabel('无法获取 API 密钥，请刷新或重新登录');
      }
    }

    initPanelSelects($('#panel-text'));
    initPanelSelects($('#panel-image'));
    fetchAndPopulateApiKeys();
  })();

  (() => {
    $$('.field-ratio').forEach(field => {
      const cards = $$('.ratio-card', field);
      const tierBtns = $$('.tier-btn', field);
      const display = $('.image-active-value', field);

      cards.forEach(card => {
        card.addEventListener('click', () => {
          cards.forEach(c => c.classList.remove('ratio-card-active'));
          card.classList.add('ratio-card-active');
          const ratio = card.querySelector('.ratio-label')?.textContent || '';
          if (display) display.textContent = ratio;
          updateCost();
        });
      });

      tierBtns.forEach(btn => {
        btn.addEventListener('click', () => {
          tierBtns.forEach(b => b.classList.remove('tier-btn-active'));
          btn.classList.add('tier-btn-active');
          updateCost();
        });
      });
    });
  })();

  (() => {
    $$('.image-range').forEach(range => {
      const field = range.closest('.field-prompt');
      if (!field) return;
      const display = $('.image-active-value', field);
      if (display) {
        const update = () => {
          display.textContent = range.value;
          updateCost();
        };
        range.addEventListener('input', update);
        update();
      }
    });

    $$('textarea.input, textarea.prompt-textarea').forEach(textarea => {
      const field = textarea.closest('.image-field');
      if (!field) return;
      const counter = $('.char-count', field);
      if (counter) {
        const update = () => {
          counter.textContent = textarea.value.length + ' 字符';
        };
        textarea.addEventListener('input', update);
        update();
      }
    });
  })();

  (() => {
    $$('.prompt-chip').forEach(chip => {
      chip.addEventListener('click', () => {
        const field = chip.closest('.image-field');
        if (!field) return;
        const textarea = $('textarea.input, textarea.prompt-textarea', field);
        if (textarea) {
          textarea.value = chip.textContent;
          textarea.dispatchEvent(new Event('input'));
          textarea.focus();
        }
      });
    });
  })();

  (() => {
    const panel = $('#panel-image');
    if (!panel) return;
    const fileInput = $('input[type="file"]', panel);
    const uploadLabel = $('.reference-upload', panel);
    const listContainer = $('.mock-reference-list', panel);
    const countDisplay = $('.image-muted', panel);
    if (!fileInput || !listContainer) return;

    let files = [];

    function updateCount() {
      if (countDisplay) countDisplay.textContent = files.length + ' / 4';
      if (uploadLabel) uploadLabel.style.display = files.length >= 4 ? 'none' : '';
    }

    function renderList() {
      listContainer.innerHTML = '';
      files.forEach((f, i) => {
        const item = document.createElement('div');
        item.className = 'reference-item';

        if (f.dataUrl) {
          const img = document.createElement('img');
          img.src = f.dataUrl;
          img.alt = f.name;
          img.className = 'reference-thumb';
          item.appendChild(img);
        }

        const nameSpan = document.createElement('span');
        nameSpan.className = 'reference-name';
        nameSpan.textContent = f.name;
        nameSpan.title = f.name;
        item.appendChild(nameSpan);

        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.className = 'reference-remove';
        removeBtn.innerHTML = '&times;';
        removeBtn.title = '移除';
        removeBtn.addEventListener('click', () => {
          files.splice(i, 1);
          renderList();
          updateCount();
        });
        item.appendChild(removeBtn);

        listContainer.appendChild(item);
      });
      updateCost();
    }

    fileInput.addEventListener('change', () => {
      const newFiles = Array.from(fileInput.files);
      const remaining = 4 - files.length;
      if (remaining <= 0) {
        showToast('最多上传 4 张参考图', 'warning');
        fileInput.value = '';
        return;
      }
      const toAdd = newFiles.slice(0, remaining);
      if (newFiles.length > remaining) {
        showToast('已达上限，仅添加前 ' + remaining + ' 张', 'warning');
      }
      let loaded = 0;
      toAdd.forEach(file => {
        if (file.size > 20 * 1024 * 1024) {
          showToast(file.name + ' 超过 20MB 限制', 'error');
          loaded++;
          return;
        }
        const reader = new FileReader();
        reader.onload = () => {
          files.push({ name: file.name, dataUrl: reader.result });
          loaded++;
          if (loaded === toAdd.length) renderList();
        };
        reader.onerror = () => {
          loaded++;
          showToast(file.name + ' 读取失败', 'error');
          if (loaded === toAdd.length) renderList();
        };
        reader.readAsDataURL(file);
      });
      fileInput.value = '';
    });

    if (uploadLabel) {
      uploadLabel.addEventListener('dragover', e => {
        e.preventDefault();
        uploadLabel.classList.add('reference-upload-hover');
      });
      uploadLabel.addEventListener('dragleave', () => {
        uploadLabel.classList.remove('reference-upload-hover');
      });
      uploadLabel.addEventListener('drop', e => {
        e.preventDefault();
        uploadLabel.classList.remove('reference-upload-hover');
        const dt = e.dataTransfer;
        if (dt.files.length) {
          fileInput.files = dt.files;
          fileInput.dispatchEvent(new Event('change'));
        }
      });
    }

    panel._getFiles = () => files;
    updateCount();
  })();

  function updateCost() {
    const PRICE_PER_IMAGE = 0.1;

    $$('.mode-panel').forEach(panel => {
      const costEl = $('.cost-value', panel);
      if (!costEl) return;

      let count = 1;

      const range = $('.image-range', panel);
      if (range) count = parseInt(range.value) || 1;

      const total = (PRICE_PER_IMAGE * count).toFixed(2);
      costEl.textContent = '$' + total;
    });
  }

  updateCost();

  // 最大重试次数：请求失败时最多重试3次
  const MAX_ATTEMPTS = 3;
  // 重试等待间隔：两次重试之间暂停15秒（单位：毫秒）
  const RETRY_BACKOFF_MS = 15000;
  // 单次 Images API 请求最长等待时间。上游长时间无响应时主动重试，避免界面卡住数分钟。
  const IMAGE_REQUEST_TIMEOUT_MS = 90000;
  // 最大并发数：同时生成的图片数量上限，避免并发过高导致接口限流
  const MAX_CONCURRENT = 10;

  function isRetryableError(err) {
    const msg = (err.message || '').toLowerCase();
    if (msg.includes('service temporarily unavailable')) return true;
    if (msg.includes('524') || msg.includes('504') || msg.includes('gateway time-out')) return true;
    if (msg.includes('timeout') || msg.includes('超时') || msg.includes('超过90秒') || msg.includes('aborted')) return true;
    if (msg.includes('origin_gateway_timeout')) return true;
    if (msg.includes('api_error') || msg.includes('server_error')) return true;
    if (/http 50[234]/.test(msg) || /http 524/.test(msg)) return true;
    return false;
  }

  function getErrorHint(err) {
    const msg = (err.message || '').toLowerCase();
    if (msg.includes('moderation_blocked') || msg.includes('content_policy_violation'))
      return '上游内容审核拦截，提示词可能包含违规内容';
    if (msg.includes('rate_limit_exceeded'))
      return '上游限速，请稍后再试';
    if (msg.includes('insufficient_quota') || msg.includes('billing_hard_limit_reached'))
      return '上游账户额度不足，请更换 API 密钥';
    if (msg.includes('model_not_found'))
      return '上游找不到指定模型，请检查模型配置';
    if (msg.includes('service temporarily unavailable'))
      return '服务暂时不可用，已自动重试';
    if (msg.includes('http 502') || msg.includes('bad gateway') || msg.includes('connection refused'))
      return '上游生图服务连接失败，系统会自动重试；如果持续失败，请切换到可用的生图账号';
    if (msg.includes('timeout') || msg.includes('超时') || msg.includes('超过90秒') || msg.includes('aborted'))
      return '上游生图服务长时间没有返回数据，已自动重试';
    if (msg.includes('524') || msg.includes('504') || msg.includes('gateway time-out'))
      return '上游网关超时，生成可能仍在进行';
    return '';
  }

  function walkForImageCall(value) {
    if (!value) return null;
    if (Array.isArray(value)) {
      for (const child of value) {
        const found = walkForImageCall(child);
        if (found) return found;
      }
      return null;
    }
    if (typeof value === 'object') {
      if (value.type === 'image_generation_call' && value.result) return value;
      for (const child of Object.values(value)) {
        const found = walkForImageCall(child);
        if (found) return found;
      }
    }
    return null;
  }

  function extractImageResult(raw) {
    let partialB64 = '';
    let partialPrompt = '';
    for (const line of raw.split(/\r?\n/)) {
      if (!line.startsWith('data: ')) continue;
      const payload = line.slice(6).trim();
      if (!payload || payload === '[DONE]') continue;
      let event;
      try { event = JSON.parse(payload); } catch { continue; }
      if (event.type === 'response.image_generation_call.partial_image' && event.partial_image_b64) {
        partialB64 = event.partial_image_b64;
        partialPrompt = event.revised_prompt || partialPrompt;
        continue;
      }
      if (event.type === 'response.output_item.done' && event.item?.type === 'image_generation_call') {
        if (event.item.result) {
          return { imageB64: event.item.result, revisedPrompt: event.item.revised_prompt || '' };
        }
        if (partialB64) {
          return { imageB64: partialB64, revisedPrompt: partialPrompt };
        }
      }
    }
    try {
      const parsed = JSON.parse(raw);
      const found = walkForImageCall(parsed);
      if (found?.result) {
        return { imageB64: found.result, revisedPrompt: found.revised_prompt || '' };
      }
    } catch {}
    if (partialB64) {
      return { imageB64: partialB64, revisedPrompt: partialPrompt };
    }
    return null;
  }

  function normalizeImagePayload(value, revisedPrompt = '') {
    if (typeof value !== 'string') return null;
    const raw = value.trim();
    if (!raw) return null;
    if (/^data:image\//i.test(raw)) {
      const comma = raw.indexOf(',');
      const b64 = comma >= 0 && /;base64/i.test(raw.slice(0, comma)) ? raw.slice(comma + 1) : '';
      return { imageURL: raw, imageB64: b64, revisedPrompt };
    }
    if (/^https?:\/\//i.test(raw) || raw.startsWith('/')) {
      return { imageURL: raw, revisedPrompt };
    }
    if (raw.length > 100 && /^[A-Za-z0-9+/_=-]+$/.test(raw)) {
      return { imageB64: raw, revisedPrompt };
    }
    return null;
  }

  function extractImageFromImagesResponse(value, inheritedPrompt = '') {
    if (!value) return null;
    if (Array.isArray(value)) {
      for (const child of value) {
        const found = extractImageFromImagesResponse(child, inheritedPrompt);
        if (found) return found;
      }
      return null;
    }
    if (typeof value === 'string') {
      return normalizeImagePayload(value, inheritedPrompt);
    }
    if (typeof value !== 'object') return null;

    const revisedPrompt = value.revised_prompt || value.revisedPrompt || inheritedPrompt || '';
    const directKeys = [
      'b64_json',
      'base64',
      'image_base64',
      'image_b64',
      'partial_image_b64',
      'result',
      'image',
      'url',
      'image_url',
      'download_url'
    ];
    for (const key of directKeys) {
      const found = extractImageFromImagesResponse(value[key], revisedPrompt);
      if (found) return found;
    }
    for (const child of Object.values(value)) {
      const found = extractImageFromImagesResponse(child, revisedPrompt);
      if (found) return found;
    }
    return null;
  }

  function describeImagesAPIResponse(data) {
    if (data == null) return '响应为空';
    if (typeof data !== 'object') return '响应类型: ' + typeof data;
    const rootKeys = Object.keys(data).slice(0, 12);
    const parts = ['根字段: ' + (rootKeys.join(', ') || '无')];
    if (Array.isArray(data.data)) {
      parts.push('data 数量: ' + data.data.length);
      const first = data.data[0];
      if (first && typeof first === 'object') {
        parts.push('首项字段: ' + (Object.keys(first).slice(0, 12).join(', ') || '无'));
      }
    }
    if (data.error?.message) parts.push('错误: ' + data.error.message);
    return parts.join('；');
  }

  function imageResultSource(result, mimeType) {
    if (!result) return '';
    if (result.imageURL) return result.imageURL;
    if (result.imageB64) return 'data:' + mimeType + ';base64,' + result.imageB64;
    return '';
  }

  async function requestResponsesAPI(baseURL, apiKey, requestBody, onProgress) {
    let lastError;
    for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
      try {
        const fullURL = baseURL + '/v1/responses';
        const maskedKey = apiKey ? (apiKey.slice(0, 8) + '****' + apiKey.slice(-4)) : 'null';
        console.groupCollapsed('[生图调试] 第 ' + attempt + ' 次请求');
        console.log('请求地址:', fullURL);
        console.log('API Key:', maskedKey);
        console.log('请求体:', JSON.stringify(requestBody, null, 2));
        console.groupEnd();

        if (attempt > 1 && onProgress) {
          onProgress('第 ' + attempt + ' 次重试中...');
        }
        const response = await fetch(fullURL, {
          method: 'POST',
          headers: {
            'Authorization': 'Bearer ' + apiKey,
            'Content-Type': 'application/json',
            'Accept': 'text/event-stream, application/json'
          },
          body: JSON.stringify(requestBody)
        });
        console.log('[生图调试] 响应状态:', response.status, response.statusText);
        if (!response.ok) {
          const errText = await response.text();
          console.error('[生图调试] 错误响应原文:', errText);
          let msg = 'HTTP ' + response.status;
          try {
            const errJson = JSON.parse(errText);
            msg = errJson.error?.message || errJson.message || msg;
            console.error('[生图调试] 错误JSON:', errJson);
          } catch {}
          const err = new Error(msg);
          err.httpStatus = response.status;
          throw err;
        }
        if (!response.body) {
          const raw = await response.text();
          console.log('[生图调试] 非流式响应原文(前2000字符):', raw.slice(0, 2000));
          const result = extractImageResult(raw);
          console.log('[生图调试] 提取结果:', result ? '成功(有imageB64)' : '失败(null)');
          return result;
        }
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let raw = '';
        let pending = '';
        while (true) {
          const { value, done } = await reader.read();
          if (done) break;
          const chunk = decoder.decode(value, { stream: true });
          raw += chunk;
          pending += chunk;
          let newline = pending.indexOf('\n');
          while (newline >= 0) {
            const line = pending.slice(0, newline).replace(/\r$/, '');
            pending = pending.slice(newline + 1);
            if (line.startsWith('data: ')) {
              const payload = line.slice(6).trim();
              if (payload && payload !== '[DONE]') {
                try {
                  const evt = JSON.parse(payload);
                  if (evt.type) {
                    console.log('[生图调试] SSE事件:', evt.type, evt.error ? '| 错误:' + JSON.stringify(evt.error) : '');
                    if (onProgress) {
                      const MAP = {
                        'response.created': '请求已创建',
                        'response.in_progress': '模型处理中',
                        'response.image_generation_call.in_progress': '图片工具已启动',
                        'response.image_generation_call.generating': '图片正在生成',
                        'response.image_generation_call.partial_image': '已收到图片数据片段',
                        'response.output_item.done': '图片生成完成',
                        'response.completed': '接口已完成'
                      };
                      const desc = MAP[evt.type];
                      if (desc) onProgress(desc);
                    }
                  }
                } catch {}
              }
            }
            newline = pending.indexOf('\n');
          }
        }
        console.log('[生图调试] SSE流总长度:', raw.length, '字符');
        console.log('[生图调试] SSE流原文(前3000字符):', raw.slice(0, 3000));
        const result = extractImageResult(raw);
        console.log('[生图调试] 提取结果:', result ? '成功(有imageB64)' : '失败(null)');
        return result;
      } catch (err) {
        lastError = err;
        console.error('[生图调试] 第 ' + attempt + ' 次请求失败:', err.message, 'httpStatus:', err.httpStatus || 'N/A');
        if (err.httpStatus === 503) throw err;
        if (attempt < MAX_ATTEMPTS && isRetryableError(err)) {
          if (onProgress) onProgress('服务暂时不可用，' + (RETRY_BACKOFF_MS / 1000) + '秒后重试 (' + attempt + '/' + MAX_ATTEMPTS + ')');
          await new Promise(r => setTimeout(r, RETRY_BACKOFF_MS));
          continue;
        }
        throw err;
      }
    }
    throw lastError;
  }

  async function readImagesAPIResponse(response, label, onProgress) {
    const contentType = (response.headers.get('content-type') || '').toLowerCase();
    const parseTextPayload = (raw) => {
      if (!raw) return null;
      const sseResult = extractImageResult(raw);
      if (sseResult?.imageB64 || sseResult?.imageURL) return sseResult;
      try {
        const data = JSON.parse(raw);
        console.log('[生图调试] ' + label + ' 返回数据结构:', Object.keys(data || {}), describeImagesAPIResponse(data));
        return extractImageFromImagesResponse(data);
      } catch {}
      return null;
    };

    if (!response.body || !contentType.includes('text/event-stream')) {
      const raw = await response.text();
      console.log('[生图调试] ' + label + ' 非流式响应(前3000字符):', raw.slice(0, 3000));
      const result = parseTextPayload(raw);
      if (result?.imageB64 || result?.imageURL) return result;
      let desc = '响应为空';
      try { desc = describeImagesAPIResponse(JSON.parse(raw)); } catch {}
      throw new Error(label + ' 未返回可用图片数据（' + desc + '）');
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let raw = '';
    let pending = '';
    let latestResult = null;
    const eventText = {
      'image_generation.started': '图片生成已开始',
      'image_generation.in_progress': '图片正在生成',
      'image_generation.partial_image': '已收到图片预览片段',
      'image_generation.completed': '图片生成完成',
      'response.created': '请求已创建',
      'response.in_progress': '模型处理中',
      'response.image_generation_call.in_progress': '图片工具已启动',
      'response.image_generation_call.generating': '图片正在生成',
      'response.image_generation_call.partial_image': '已收到图片数据片段',
      'response.output_item.done': '图片生成完成',
      'response.completed': '接口已完成'
    };

    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      const chunk = decoder.decode(value, { stream: true });
      raw += chunk;
      pending += chunk;
      let newline = pending.indexOf('\n');
      while (newline >= 0) {
        const line = pending.slice(0, newline).replace(/\r$/, '');
        pending = pending.slice(newline + 1);
        if (line.startsWith('data: ')) {
          const payload = line.slice(6).trim();
          if (payload && payload !== '[DONE]') {
            try {
              const evt = JSON.parse(payload);
              if (evt.type && eventText[evt.type] && onProgress) onProgress(eventText[evt.type]);
              const found = extractImageFromImagesResponse(evt);
              if (found?.imageB64 || found?.imageURL) latestResult = found;
            } catch {}
          }
        }
        newline = pending.indexOf('\n');
      }
    }

    console.log('[生图调试] ' + label + ' SSE流总长度:', raw.length, '字符');
    console.log('[生图调试] ' + label + ' SSE流原文(前3000字符):', raw.slice(0, 3000));
    if (latestResult?.imageB64 || latestResult?.imageURL) return latestResult;
    const result = extractImageResult(raw);
    if (result?.imageB64 || result?.imageURL) return result;
    throw new Error(label + ' 未返回可用图片数据（SSE事件中未找到图片字段）');
  }

  async function requestImagesWithRetry(label, requestFactory, onProgress) {
    let lastError;
    for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
      let timeoutId = null;
      let controller = null;
      try {
        if (attempt > 1 && onProgress) {
          onProgress(label + ' 第 ' + attempt + ' 次重试中...');
        }
        const { fullURL, fetchOptions } = requestFactory();
        if (typeof AbortController !== 'undefined') {
          controller = new AbortController();
          timeoutId = setTimeout(() => {
            try { controller.abort(); } catch {}
          }, IMAGE_REQUEST_TIMEOUT_MS);
          fetchOptions.signal = controller.signal;
        }
        console.log('[生图调试] ' + label + ' 请求:', fullURL, 'attempt:', attempt);
        const response = await fetch(fullURL, fetchOptions);
        console.log('[生图调试] ' + label + ' 响应状态:', response.status);
        if (!response.ok) {
          const errText = await response.text();
          console.error('[生图调试] ' + label + ' 错误:', errText);
          let msg = 'HTTP ' + response.status;
          try {
            const errJson = JSON.parse(errText);
            msg = errJson.error?.message || errJson.message || msg;
          } catch {}
          const err = new Error(msg);
          err.httpStatus = response.status;
          throw err;
        }
        const result = await readImagesAPIResponse(response, label, onProgress);
        if (timeoutId) clearTimeout(timeoutId);
        return result;
      } catch (err) {
        if (timeoutId) clearTimeout(timeoutId);
        if ((err && err.name === 'AbortError') || (controller && controller.signal && controller.signal.aborted)) {
          err = new Error(label + ' 单次请求超过90秒无响应');
          err.httpStatus = 504;
        }
        lastError = err;
        console.error('[生图调试] ' + label + ' 第 ' + attempt + ' 次请求失败:', err.message, 'httpStatus:', err.httpStatus || 'N/A');
        if (attempt < MAX_ATTEMPTS && isRetryableError(err)) {
          if (onProgress) onProgress(label + ' 服务暂时不可用，' + (RETRY_BACKOFF_MS / 1000) + '秒后重试 (' + attempt + '/' + MAX_ATTEMPTS + ')');
          await new Promise(r => setTimeout(r, RETRY_BACKOFF_MS));
          continue;
        }
        throw err;
      }
    }
    throw lastError;
  }

  async function requestImagesAPI(baseURL, apiKey, requestBody, onProgress) {
    const fullURL = baseURL + '/v1/images/generations';
    const maskedKey = apiKey ? (apiKey.slice(0, 8) + '****' + apiKey.slice(-4)) : 'null';
    console.log('[生图调试] 回退到 Images API:', fullURL, 'API Key:', maskedKey);
    if (onProgress) onProgress('正在通过备用接口生成...');
    return requestImagesWithRetry('Images API', () => {
      const streamBody = { ...requestBody, stream: true, partial_images: 1 };
      return {
        fullURL,
        fetchOptions: {
          method: 'POST',
          headers: {
            'Authorization': 'Bearer ' + apiKey,
            'Content-Type': 'application/json',
            'Accept': 'text/event-stream, application/json'
          },
          body: JSON.stringify(streamBody)
        }
      };
    }, onProgress);
  }

  async function requestImagesEditAPI(baseURL, apiKey, formData, onProgress) {
    const fullURL = baseURL + '/v1/images/edits';
    console.log('[生图调试] 回退到 Images Edit API:', fullURL);
    if (onProgress) onProgress('正在通过备用接口生成...');
    if (!formData.has('stream')) formData.append('stream', 'true');
    if (!formData.has('partial_images')) formData.append('partial_images', '1');
    return requestImagesWithRetry('Images Edit API', () => ({
      fullURL,
      fetchOptions: {
        method: 'POST',
        headers: {
          'Authorization': 'Bearer ' + apiKey,
          'Accept': 'text/event-stream, application/json'
        },
        body: formData
      }
    }), onProgress);
  }

  async function generateWithConcurrency(tasks, maxConcurrent, onProgress) {
    const results = new Array(tasks.length);
    let completedCount = 0;
    let nextIndex = 0;

    function updateProgress() {
      if (onProgress) {
        onProgress('已完成 ' + completedCount + '/' + tasks.length + ' 张图片');
      }
    }

    async function runTask(index) {
      const task = tasks[index];
      try {
        results[index] = await task();
        completedCount++;
        updateProgress();
      } catch (err) {
        results[index] = { error: err, index };
        completedCount++;
        updateProgress();
      }
    }

    const workers = [];
    const concurrentCount = Math.min(maxConcurrent, tasks.length);

    for (let i = 0; i < concurrentCount; i++) {
      workers.push((async () => {
        while (nextIndex < tasks.length) {
          const currentIndex = nextIndex++;
          await runTask(currentIndex);
        }
      })());
    }

    await Promise.all(workers);
    return results;
  }

  function getSelectedApiKey(panel) {
    const trigger = panel.querySelector('.select-trigger');
    if (!trigger) return null;
    const field = trigger.closest('.image-field');
    if (!field) return null;
    const label = field.querySelector('.input-label');
    if (!label) return null;
    if (!label.textContent.trim().includes('API') && !label.textContent.trim().includes('密钥')) return null;
    const selectedValue = trigger.dataset.selectValue || '';
    const selectedId = trigger.dataset.selectId || '';
    const selectedLabel = trigger.querySelector('.select-value')?.textContent?.trim() || trigger.dataset.selectLabel || '';
    if (!selectedValue && !selectedLabel) return null;
    const options = SELECT_OPTIONS['api-key'] || [];
    const match = options.find(o => selectedId && o.id === selectedId)
      || options.find(o => selectedValue && o.value === selectedValue)
      || options.find(o => o.label === selectedLabel);
    console.log('[生图调试] 选中的密钥:', { selectedLabel, selectedId, matched: !!match, allOptions: options.map(o => o.label) });
    return match ? match.value : null;
  }

  function getPanelParams(panel) {
    const textarea = panel.querySelector('textarea.input, textarea.prompt-textarea');
    const prompt = textarea ? textarea.value.trim() : '';
    const modelSelect = panel.querySelectorAll('.select-trigger');
    let model = 'gpt-image-2';
    let quality = 'auto';
    let outputFormat = 'png';
    modelSelect.forEach(trigger => {
      const f = trigger.closest('.image-field');
      if (!f) return;
      const lbl = f.querySelector('.input-label');
      if (!lbl) return;
      const txt = lbl.textContent.trim();
      const val = trigger.querySelector('.select-value')?.textContent?.trim() || '';
      if (txt.includes('模型')) model = SELECT_OPTIONS['model']?.find(o => o.label === val)?.value || model;
      if (txt.includes('质量')) quality = SELECT_OPTIONS['quality']?.find(o => o.label === val)?.value || quality;
      if (txt.includes('输出格式')) outputFormat = SELECT_OPTIONS['format']?.find(o => o.label === val)?.value || outputFormat;
    });
    const activeCard = panel.querySelector('.ratio-card-active');
    const ratio = activeCard?.querySelector('.ratio-label')?.textContent || '自动生成';
    
    const tierBtn = panel.querySelector('.tier-btn-active');
    const resolutionTier = tierBtn?.dataset?.tier || 'auto';
    
    console.log('[生图调试] ===== 参数解析 =====');
    console.log('[生图调试] 画面比例:', ratio);
    console.log('[生图调试] 分辨率档位:', resolutionTier);
    console.log('[生图调试] 模型:', model);
    console.log('[生图调试] 质量:', quality);
    console.log('[生图调试] 输出格式:', outputFormat);
    
    const sizeIntent = buildSizeIntent(ratio, resolutionTier);
    console.log('[生图调试] SizeIntent:', JSON.stringify(sizeIntent, null, 2));
    
    const range = panel.querySelector('.image-range');
    const count = range ? parseInt(range.value) || 1 : 1;
    return { prompt, model, quality, outputFormat, sizeIntent, ratio, resolutionTier, count };
  }

  async function textToImage({ prompt, baseURL, apiKey, imageModel, sizeIntent, quality, outputFormat, onProgress }) {
    const tool = {
      type: 'image_generation',
      model: imageModel || 'gpt-image-2',
      action: 'generate',
      size: 'auto',
      quality: quality || 'auto',
      output_format: outputFormat || 'png',
      moderation: 'low',
      partial_images: 0
    };
    
    applySizeIntentToTool(tool, sizeIntent, imageModel);
    const enhancedPrompt = enhancePromptWithSizeIntent(prompt, sizeIntent);
    
    console.log('[生图调试] ===== 最终请求 =====');
    console.log('[生图调试] tool.size:', tool.size);
    console.log('[生图调试] tool.model:', tool.model);
    console.log('[生图调试] tool.quality:', tool.quality);
    console.log('[生图调试] enhancedPrompt:', enhancedPrompt);
    
    const body = {
      model: 'gpt-5.5',
      instructions: 'You are a tool runner. Pass the user prompt to image_generation VERBATIM. DO NOT rewrite, expand, polish, or revise it in any way. Use the exact text the user gave.',
      input: [{ role: 'user', content: [{ type: 'input_text', text: enhancedPrompt }] }],
      tools: [tool],
      tool_choice: { type: 'image_generation' },
      reasoning: { effort: 'xhigh' },
      store: false,
      stream: true
    };
    const imagesBody = {
      model: imageModel || "gpt-image-2",
      prompt: enhancedPrompt,
      n: 1,
      size: tool.size === "auto" ? "1024x1024" : tool.size,
      quality: quality || "auto",
      output_format: outputFormat || "png",
      response_format: "b64_json"
    };
    return requestImagesAPI(baseURL, apiKey, imagesBody, onProgress);
  }

  async function imageToImage({ prompt, sourceImages, baseURL, apiKey, imageModel, sizeIntent, quality, outputFormat, onProgress }) {
    const tool = {
      type: 'image_generation',
      model: imageModel || 'gpt-image-2',
      action: 'edit',
      size: 'auto',
      quality: quality || 'auto',
      output_format: outputFormat || 'png',
      moderation: 'low',
      partial_images: 0
    };
    
    applySizeIntentToTool(tool, sizeIntent, imageModel);
    const enhancedPrompt = enhancePromptWithSizeIntent(prompt, sizeIntent);
    
    console.log('[生图调试] 图生图应用SizeIntent后tool:', { size: tool.size });
    
    const content = [{ type: 'input_text', text: enhancedPrompt }];
    for (const dataURL of sourceImages) {
      content.push({ type: 'input_image', image_url: dataURL });
    }
    const body = {
      model: 'gpt-5.5',
      instructions: 'You are a tool runner. Pass the user prompt to image_generation VERBATIM. DO NOT rewrite, expand, polish, or revise it in any way. Use the exact text the user gave.',
      input: [{ role: 'user', content }],
      tools: [tool],
      tool_choice: { type: 'image_generation' },
      reasoning: { effort: 'xhigh' },
      store: false,
      stream: true
    };
    const form = new FormData();
    for (let i = 0; i < sourceImages.length; i++) {
      const dataURL = sourceImages[i];
      const base64Part = dataURL.slice(dataURL.indexOf(",") + 1);
      const mimeType = dataURL.slice(5, dataURL.indexOf(";")) || "image/png";
      const ext = mimeType.split("/")[1] || "png";
      const binary = atob(base64Part);
      const bytes = new Uint8Array(binary.length);
      for (let j = 0; j < binary.length; j++) bytes[j] = binary.charCodeAt(j);
      const blob = new Blob([bytes], { type: mimeType });
      form.append(i === 0 ? "image" : "image[]", blob, "source-" + (i + 1) + "." + ext);
    }
    form.append("prompt", enhancedPrompt);
    form.append("model", imageModel || "gpt-image-2");
    form.append("n", "1");
    form.append("size", tool.size === "auto" ? "1024x1024" : tool.size);
    form.append("quality", quality || "auto");
    form.append("output_format", outputFormat || "png");
    form.append("response_format", "b64_json");
    return requestImagesEditAPI(baseURL, apiKey, form, onProgress);
  }

  (() => {
    function getFormatMime(fmt) {
      return fmt === 'jpeg' ? 'image/jpeg' : fmt === 'webp' ? 'image/webp' : 'image/png';
    }


    function createHistoryThumbnail(dataURL) {
      return new Promise(resolve => {
        const img = new Image();
        img.onload = () => {
          const maxSide = 360;
          const ratio = Math.min(1, maxSide / Math.max(img.naturalWidth || img.width, img.naturalHeight || img.height));
          const width = Math.max(1, Math.round((img.naturalWidth || img.width) * ratio));
          const height = Math.max(1, Math.round((img.naturalHeight || img.height) * ratio));
          const canvas = document.createElement('canvas');
          canvas.width = width;
          canvas.height = height;
          const ctx = canvas.getContext('2d');
          if (!ctx) {
            resolve(dataURL);
            return;
          }
          ctx.drawImage(img, 0, 0, width, height);
          try {
            resolve(canvas.toDataURL('image/webp', 0.72));
          } catch (e) {
            try { resolve(canvas.toDataURL('image/png')); }
            catch { resolve(dataURL); }
          }
        };
        img.onerror = () => resolve(dataURL);
        img.src = dataURL;
      });
    }

    // Global generation status indicator (visible across all tabs)
    (function initGenIndicator() {
      const style = document.createElement('style');
      style.textContent = '.gen-global-indicator{display:none;align-items:center;gap:8px;padding:8px 14px;margin:0 0 8px;border-radius:8px;background:#eff6ff;border:1px solid #bfdbfe;color:#1e40af;font-size:13px;cursor:pointer;transition:all .2s;animation:gen-gi-slide-in .25s ease}[data-theme="dark"] .gen-global-indicator{background:#1e293b;border-color:#334155;color:#93c5fd}.gen-global-indicator.active{display:flex}.gen-global-indicator .gen-gi-spinner{width:16px;height:16px;border:2px solid currentColor;border-top-color:transparent;border-radius:50%;animation:gen-gi-spin .8s linear infinite;flex-shrink:0}.gen-global-indicator .gen-gi-check{width:16px;height:16px;flex-shrink:0}.gen-global-indicator .gen-gi-text{flex:1;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.gen-global-indicator .gen-gi-btn{background:none;border:1px solid currentColor;border-radius:4px;padding:2px 8px;font-size:12px;cursor:pointer;color:inherit;flex-shrink:0;transition:background .15s}.gen-global-indicator .gen-gi-btn:hover{background:rgba(0,0,0,.08)}[data-theme="dark"] .gen-global-indicator .gen-gi-btn:hover{background:rgba(255,255,255,.1)}.gen-spinner{animation:gen-gi-spin .8s linear infinite!important;will-change:transform}.gen-btn-spin{animation:gen-gi-spin 1s linear infinite!important;transform-origin:center;will-change:transform}@keyframes gen-gi-spin{to{transform:rotate(360deg)}}@keyframes gen-gi-slide-in{from{opacity:0;transform:translateY(-6px)}to{opacity:1;transform:translateY(0)}}';
      document.head.appendChild(style);

      const indicator = document.createElement('div');
      indicator.className = 'gen-global-indicator';
      indicator.innerHTML = '<span class="gen-gi-spinner"></span><span class="gen-gi-text"></span>';
      const shell = document.querySelector('.image-shell');
      const tabs = document.querySelector('.image-tabs');
      if (shell && tabs) {
        shell.insertBefore(indicator, tabs);
      }

      let hideTimer = null;
      let targetMode = 'text';

      window._genIndicator = {
        show(text, mode) {
          if (hideTimer) { clearTimeout(hideTimer); hideTimer = null; }
          targetMode = mode || 'text';
          indicator.querySelector('.gen-gi-spinner').outerHTML = '<span class="gen-gi-spinner"></span>';
          indicator.querySelector('.gen-gi-text').textContent = text || '生成中...';
          const existingBtn = indicator.querySelector('.gen-gi-btn');
          if (existingBtn) existingBtn.remove();
          indicator.classList.add('active');
          indicator.onclick = null;
        },
        update(text) {
          const textEl = indicator.querySelector('.gen-gi-text');
          if (textEl) textEl.textContent = text;
        },
        done(text, mode) {
          targetMode = mode || targetMode;
          indicator.querySelector('.gen-gi-spinner').outerHTML = '<svg class="gen-gi-check" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>';
          const textEl = indicator.querySelector('.gen-gi-text');
          if (textEl) textEl.textContent = text || '生成完成';
          const existingBtn = indicator.querySelector('.gen-gi-btn');
          if (existingBtn) existingBtn.remove();
          const btn = document.createElement('button');
          btn.type = 'button';
          btn.className = 'gen-gi-btn';
          btn.textContent = '查看结果';
          btn.addEventListener('click', function(e) {
            e.stopPropagation();
            window._genIndicator.hide();
            const tab = document.querySelector('.image-tab[data-mode="' + targetMode + '"]');
            if (tab) tab.click();
          });
          indicator.appendChild(btn);
          indicator.onclick = function() {
            window._genIndicator.hide();
            const tab = document.querySelector('.image-tab[data-mode="' + targetMode + '"]');
            if (tab) tab.click();
          };
          if (hideTimer) clearTimeout(hideTimer);
          hideTimer = setTimeout(function() { window._genIndicator.hide(); }, 15000);
        },
        error(text) {
          indicator.querySelector('.gen-gi-spinner').outerHTML = '<svg class="gen-gi-check" viewBox="0 0 24 24" fill="none" stroke="#ef4444" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z"/></svg>';
          const textEl = indicator.querySelector('.gen-gi-text');
          if (textEl) { textEl.textContent = text || '生成失败'; textEl.style.color = '#ef4444'; }
          if (hideTimer) clearTimeout(hideTimer);
          hideTimer = setTimeout(function() { window._genIndicator.hide(); }, 8000);
        },
        hide() {
          indicator.classList.remove('active');
          if (hideTimer) { clearTimeout(hideTimer); hideTimer = null; }
        }
      };
    })();
    $$('.image-submit').forEach(btn => {
      btn.addEventListener('click', async () => {
        const panel = btn.closest('.mode-panel');
        if (!panel) return;

        const { prompt, model, quality, outputFormat, sizeIntent, ratio, resolutionTier, count } = getPanelParams(panel);
        if (!prompt) {
          showToast('请输入提示词', 'warning');
          const ta = panel.querySelector('textarea.input, textarea.prompt-textarea');
          if (ta) ta.focus();
          return;
        }

        const apiKey = getSelectedApiKey(panel);
        if (!apiKey) {
          showToast('请选择 API 密钥', 'warning');
          return;
        }

        const mode = panel.dataset.panel;
        let sourceImages = [];
        if (mode === 'image') {
          const files = panel._getFiles ? panel._getFiles() : [];
          if (files.length === 0) {
            showToast('请先上传参考图', 'warning');
            return;
          }
          sourceImages = files.map(f => f.dataUrl);
        }

        const canvas = panel.querySelector('.image-canvas');
        if (!canvas) return;

        btn.disabled = true;
        btn.dataset.originalHTML = btn.innerHTML;
        btn.innerHTML = '<svg class="h-5 w-5 gen-btn-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182"/></svg> 生成中...';

        const startTime = Date.now();
        let timerInterval = null;

        canvas.innerHTML = '<div class="gen-loading"><div class="gen-spinner"></div><p class="gen-loading-text">正在并行生成 ' + count + ' 张图片，请稍候...</p><p class="gen-loading-hint gen-timer">已用时 0.0 秒</p><p class="gen-progress-text">准备中...</p></div>';
        if (window._genIndicator) window._genIndicator.show('正在生成 ' + count + ' 张图片...', mode);

        const timerEl = canvas.querySelector('.gen-timer');
        const progressEl = canvas.querySelector('.gen-progress-text');

        timerInterval = setInterval(() => {
          if (timerEl) {
            const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
            timerEl.textContent = '已用时 ' + elapsed + ' 秒';
          }
        }, 100);

        function onProgress(desc) {
          if (progressEl) progressEl.textContent = desc;
          if (window._genIndicator) window._genIndicator.update(desc);
        }

        const baseURL = (typeof window !== 'undefined' ? window.location.origin : '') || iframeState.srcHost || '';

        try {
          const tasks = [];
          for (let i = 0; i < count; i++) {
            tasks.push(async () => {
              if (mode === 'image') {
                return imageToImage({ prompt, sourceImages, baseURL, apiKey, imageModel: model, sizeIntent, quality, outputFormat, onProgress: (desc) => onProgress('图片 ' + (i + 1) + '/' + count + ': ' + desc) });
              } else {
                return textToImage({ prompt, baseURL, apiKey, imageModel: model, sizeIntent, quality, outputFormat, onProgress: (desc) => onProgress('图片 ' + (i + 1) + '/' + count + ': ' + desc) });
              }
            });
          }

          const results = await generateWithConcurrency(tasks, MAX_CONCURRENT, onProgress);

          if (timerInterval) clearInterval(timerInterval);
          const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);

          const successfulResults = [];
          const failedIndices = [];
          results.forEach((result, index) => {
            if (result && (result.imageB64 || result.imageURL)) {
              successfulResults.push({ ...result, index });
            } else {
              failedIndices.push(index);
              console.error('[生图调试] 图片 ' + (index + 1) + ' 生成失败:', result?.error || '无图片数据');
            }
          });

          if (successfulResults.length === 0) {
            const firstFailure = results.find(result => result && result.error);
            const detail = firstFailure?.error?.message || "所有图片生成均失败，请稍后重试";
            throw new Error(detail);
          }

          const mimeType = getFormatMime(outputFormat);
          const now = new Date();
          const timeStr = String(now.getHours()).padStart(2, '0') + ':' + String(now.getMinutes()).padStart(2, '0');
          const headTitle = mode === 'image' ? '参考图改绘结果' : '本次生成结果';
          const failNote = failedIndices.length > 0 ? '（' + failedIndices.length + ' 张生成失败）' : '';
          const headDesc = mode === 'image'
            ? '基于参考图生成 ' + successfulResults.length + '/' + count + ' 张，耗时 ' + elapsed + ' 秒' + failNote
            : '并行生成 ' + successfulResults.length + '/' + count + ' 张，耗时 ' + elapsed + ' 秒' + failNote;

          let imagesHTML = '';
          successfulResults.forEach((result, idx) => {
            const imageSrc = imageResultSource(result, mimeType);
            const badge = String(idx + 1).padStart(2, '0');
            const publishDisabled = result.imageB64 ? '' : ' disabled title="URL 图片暂不支持直接发布到画廊"';
            imagesHTML += '<div class="result-image-wrap" data-index="' + idx + '"><img src="' + escapeHTML(imageSrc) + '" alt="生成图片 ' + badge + '" loading="lazy" class="result-image" data-action="preview"><span class="result-badge">' + badge + '</span><div class="result-actions"><button type="button" class="result-action-btn" data-action="publish" data-index="' + idx + '"' + publishDisabled + '><svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 16.5V9.75m0 0l-3 3m3-3l3 3M6.75 19.5A2.25 2.25 0 014.5 17.25V6.75A2.25 2.25 0 016.75 4.5h10.5a2.25 2.25 0 012.25 2.25v10.5a2.25 2.25 0 01-2.25 2.25H6.75z"/></svg></button><button type="button" class="result-action-btn" data-action="download" data-index="' + idx + '" title="下载"><svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg></button><button type="button" class="result-action-btn" data-action="preview" data-index="' + idx + '" title="放大预览"><svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607zM10.5 7.5v6m3-3h-6"/></svg></button></div></div>';
          });

          const previewClass = successfulResults.length === 1 ? 'gen-single-preview' : 'gen-multi-preview gen-count-' + Math.min(successfulResults.length, 5);
          canvas.innerHTML = '<div class="gen-results"><div class="gen-result-head"><div><h3>' + headTitle + '</h3><p>' + headDesc + '</p></div><span class="gen-status">完成 · ' + timeStr + '</span></div><div class="' + previewClass + '">' + imagesHTML + '</div></div>';
          if (window._genIndicator) window._genIndicator.done(successfulResults.length + '/' + count + ' 张图片生成完成', mode);

          const previewImages = canvas.querySelectorAll('.result-image[data-action="preview"], .result-action-btn[data-action="preview"]');
          previewImages.forEach(el => {
            el.addEventListener('click', (e) => {
              e.stopPropagation();
              const wrap = el.closest('.result-image-wrap');
              const idx = parseInt(wrap?.dataset?.index ?? el.dataset?.index ?? '0');
              const result = successfulResults[idx];
              if (result) {
                const imageSrc = imageResultSource(result, mimeType);
                showImageLightbox(imageSrc);
              }
            });
          });

          canvas.querySelectorAll('.result-action-btn[data-action="download"]').forEach(downloadBtn => {
            downloadBtn.addEventListener('click', (e) => {
              e.stopPropagation();
              const idx = parseInt(downloadBtn.dataset.index);
              const result = successfulResults[idx];
              if (result) {
                const imageSrc = imageResultSource(result, mimeType);
                const a = document.createElement('a');
                a.href = imageSrc;
                a.download = 'generated-image-' + Date.now() + '-' + (idx + 1) + '.' + outputFormat;
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
                showToast('开始下载图片 ' + (idx + 1), 'success');
              }
            });
          });

          canvas.querySelectorAll('.result-action-btn[data-action="publish"]').forEach(publishBtn => {
            publishBtn.addEventListener('click', (e) => {
              e.stopPropagation();
              const idx = parseInt(publishBtn.dataset.index);
              const result = successfulResults[idx];
              if (!result || !result.imageB64) {
                showToast('当前结果不是 base64 图片，暂不能发布到画廊', 'error');
                return;
              }
              const dataURL = imageResultSource(result, mimeType);
              publishGalleryItem(dataURL, {
                prompt,
                revisedPrompt: result.revisedPrompt || '',
                model,
                size: sizeIntent.size || '',
                quality,
                outputFormat,
                mode
              }, publishBtn);
            });
          });

          try {
            const baseTimestamp = Date.now();
            for (let idx = successfulResults.length - 1; idx >= 0; idx--) {
              const result = successfulResults[idx];
              const dataURL = imageResultSource(result, mimeType);
              const historyId = baseTimestamp + idx;
              const imageStored = await storeHistoryImage(historyId, dataURL);
              saveToHistory({
                id: historyId,
                image_key: imageStored ? String(historyId) : '',
                image: imageStored ? '' : dataURL,
                mode: mode === 'image' ? '图生图' : '文生图',
                prompt: prompt,
                count: 1,
                ratio: ratio,
                format: outputFormat.toUpperCase(),
                cost: panel.querySelector('.cost-value')?.textContent || '$0.00',
                timestamp: now.toISOString(),
                thumbnail: await createHistoryThumbnail(dataURL),
                model: model,
                size: sizeIntent.size || '',
                quality: quality,
                outputFormat: outputFormat,
                generationMode: mode
              });
            }
          } catch (historyErr) {
            console.warn('[历史记录] 保存或刷新失败:', historyErr.message || historyErr);
            showToast('图片已生成，但历史记录保存失败', 'warning');
          }

        } catch (err) {
          if (timerInterval) clearInterval(timerInterval);
          const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
          const hint = getErrorHint(err);
          const hintHtml = hint ? '<p class="gen-loading-hint" style="color:#f59e0b">' + hint + '</p>' : '';
          canvas.innerHTML = '<div class="gen-loading"><div class="image-empty-icon" style="background:linear-gradient(135deg,#fecaca,#fef2f2);color:#ef4444"><svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z"/></svg></div><h3>生成失败</h3><p>' + (err.message || '未知错误') + '</p>' + hintHtml + '<p class="gen-loading-hint">耗时 ' + elapsed + ' 秒</p></div>';
          showToast('生成失败: ' + (err.message || '未知错误'), 'error');
          if (window._genIndicator) window._genIndicator.error(err.message || '生成失败');
        } finally {
          btn.disabled = false;
          btn.innerHTML = btn.dataset.originalHTML || '<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456z"/></svg> 生成图片';
        }
      });
    });

    $$('.image-canvas').forEach(canvas => {
      if (!canvas.querySelector('.gen-results') && !canvas.querySelector('.mock-results')) {
        canvas.innerHTML = '<div class="image-empty"><div class="image-empty-icon"><svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M2.25 15.75l5.159-5.159a2.25 2.25 0 013.182 0l5.159 5.159m-1.5-1.5l1.409-1.409a2.25 2.25 0 013.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 001.5-1.5V6a1.5 1.5 0 00-1.5-1.5H3.75A1.5 1.5 0 002.25 6v12a1.5 1.5 0 001.5 1.5zm10.5-11.25h.008v.008h-.008V8.25zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0z"/></svg></div><h3>等待生成</h3><p>输入提示词并点击「生成图片」按钮开始创作。</p></div>';
      }
    });
  })();

  const galleryState = {
    page: 1,
    pageSize: 20,
    loading: false,
    done: false,
    initialized: false
  };

  async function createGalleryThumbnail(dataURL) {
    return new Promise(resolve => {
      const img = new Image();
      img.onload = () => {
        const maxSide = 480;
        const ratio = Math.min(1, maxSide / Math.max(img.naturalWidth || img.width, img.naturalHeight || img.height));
        const width = Math.max(1, Math.round((img.naturalWidth || img.width) * ratio));
        const height = Math.max(1, Math.round((img.naturalHeight || img.height) * ratio));
        const canvas = document.createElement('canvas');
        canvas.width = width;
        canvas.height = height;
        const ctx = canvas.getContext('2d');
        if (!ctx) {
          resolve(dataURL);
          return;
        }
        ctx.drawImage(img, 0, 0, width, height);
        try { resolve(canvas.toDataURL('image/jpeg', 0.78)); }
        catch { resolve(dataURL); }
      };
      img.onerror = () => resolve(dataURL);
      img.src = dataURL;
    });
  }

  async function publishGalleryItem(dataURL, meta, button) {
    if (!iframeState.token) {
      showToast('请先登录后再发布', 'warning');
      return;
    }
    if (!dataURL || !dataURL.startsWith('data:image/')) {
      showToast('无法发布此图片', 'error');
      return;
    }
    const originalText = button ? button.innerHTML : '';
    if (button) {
      button.disabled = true;
      button.innerHTML = '<span>发布中</span>';
    }
    try {
      const thumbData = await createGalleryThumbnail(dataURL);
      const resp = await callSub2APIJSON('/api/v1/gallery/items', {
        image_data: dataURL,
        thumb_data: thumbData,
        prompt: meta.prompt || '',
        revised_prompt: meta.revisedPrompt || '',
        model: meta.model || '',
        size: meta.size || '',
        quality: meta.quality || '',
        format: meta.outputFormat || 'png',
        mode: meta.mode || 'text'
      }, { timeoutMs: 30000 });
      showToast('已发布到画廊', 'success');
      if (button) {
        button.dataset.published = 'true';
        button.innerHTML = '<span>已发布</span>';
      }
      resetGallery();
      await loadGalleryPage();
      return resp;
    } catch (err) {
      showToast('发布失败: ' + (err.message || '未知错误'), 'error');
      if (button) {
        button.disabled = false;
        button.innerHTML = originalText;
      }
    }
  }

  function resetGallery() {
    galleryState.page = 1;
    galleryState.done = false;
    const grid = $('[data-gallery-grid]');
    if (grid) grid.innerHTML = '';
  }

  async function loadGalleryPage() {
    const grid = $('[data-gallery-grid]');
    const moreBtn = $('[data-gallery-more]');
    if (!grid || galleryState.loading || galleryState.done) return;
    galleryState.loading = true;
    if (moreBtn) {
      moreBtn.disabled = true;
      moreBtn.textContent = '加载中...';
    }
    try {
      const resp = await callSub2API('/api/v1/gallery/items?page=' + galleryState.page + '&page_size=' + galleryState.pageSize, { timeoutMs: 10000 });
      const payload = resp?.data || {};
      const items = payload.items || [];
      if (galleryState.page === 1 && items.length === 0) {
        grid.innerHTML = '<div class="history-empty gallery-empty"><div class="image-empty-icon"><svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h12A2.25 2.25 0 0120.25 6v12A2.25 2.25 0 0118 20.25H6A2.25 2.25 0 013.75 18V6zM8.25 9.75h.008v.008H8.25V9.75zm2.25 0h.008v.008H10.5V9.75zm2.25 0h.008v.008h-.008V9.75z"/></svg></div><h3>暂无公开图片</h3><p>生成图片后可手动发布到这里。</p></div>';
      } else {
        const frag = document.createDocumentFragment();
        items.forEach(item => frag.appendChild(createGalleryCard(item)));
        grid.appendChild(frag);
      }
      galleryState.done = galleryState.page >= (payload.pages || 1) || items.length === 0;
      galleryState.page += 1;
    } catch (err) {
      showToast('画廊加载失败: ' + (err.message || '未知错误'), 'error');
    } finally {
      galleryState.loading = false;
      if (moreBtn) {
        moreBtn.disabled = galleryState.done;
        moreBtn.textContent = galleryState.done ? '没有更多' : '加载更多';
      }
    }
  }

  function createGalleryCard(item) {
    const article = document.createElement('article');
    article.className = 'gallery-card';
    const prompt = item.prompt || '';
    const createdAt = item.created_at ? new Date(item.created_at) : null;
    const timeLabel = createdAt && !Number.isNaN(createdAt.getTime()) ? createdAt.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) : '';
    article.innerHTML = '<button type="button" class="gallery-thumb-btn"><img src="' + escapeHTML(item.thumb_url || item.image_url || '') + '" alt="画廊图片" loading="lazy"></button><div class="gallery-card-body"><button type="button" class="gallery-prompt" title="点击复制提示词">' + escapeHTML(prompt) + '</button><div class="gallery-meta"><span>' + escapeHTML(item.model || '-') + '</span><span>' + escapeHTML(item.size || '-') + '</span><span>' + escapeHTML(item.quality || '-') + '</span></div><div class="gallery-foot"><span>' + escapeHTML(item.user_name || '用户') + ' · ' + escapeHTML(timeLabel) + '</span><button type="button" class="gallery-use-btn">重新生成</button></div></div>';
    const imgBtn = article.querySelector('.gallery-thumb-btn');
    imgBtn.addEventListener('click', () => showImageLightbox(item.image_url || item.thumb_url || ''));
    article.querySelector('.gallery-prompt').addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(prompt);
        showToast('提示词已复制', 'success');
      } catch {
        showToast('复制失败', 'error');
      }
    });
    article.querySelector('.gallery-use-btn').addEventListener('click', () => {
      const textPanel = $('#panel-text');
      const textarea = textPanel?.querySelector('textarea.input, textarea.prompt-textarea');
      if (textarea) {
        textarea.value = prompt;
        textarea.dispatchEvent(new Event('input'));
      }
      const tab = $('.image-tab[data-mode="text"]');
      if (tab) tab.click();
    });
    return article;
  }

  const galleryMoreBtn = $('[data-gallery-more]');
  if (galleryMoreBtn) {
    galleryMoreBtn.addEventListener('click', loadGalleryPage);
  }

  const galleryTab = $('.image-tab[data-mode="gallery"]');
  if (galleryTab) {
    galleryTab.addEventListener('click', () => {
      if (!galleryState.initialized) {
        galleryState.initialized = true;
        loadGalleryPage();
      }
    });
  }

  function getImageExtension(imageSrc, fallbackFormat) {
    const dataMatch = String(imageSrc || '').match(/^data:image\/([^;]+)/i);
    const raw = (dataMatch ? dataMatch[1] : fallbackFormat || 'png').toLowerCase();
    if (raw === 'jpeg') return 'jpg';
    return raw.replace(/[^a-z0-9]/g, '') || 'png';
  }

  function downloadImageSrc(imageSrc, filenamePrefix, fallbackFormat) {
    if (!imageSrc) {
      showToast('没有可下载的图片数据', 'warning');
      return;
    }
    const a = document.createElement('a');
    a.href = imageSrc;
    a.download = (filenamePrefix || 'image') + '-' + Date.now() + '.' + getImageExtension(imageSrc, fallbackFormat);
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    showToast('开始下载图片', 'success');
  }

  function showImageLightbox(imageSrc, options = {}) {
    let overlay = document.getElementById('image-lightbox');
    if (overlay) {
      overlay.remove();
    }

    overlay = document.createElement('div');
    overlay.id = 'image-lightbox';
    overlay.className = 'lightbox-overlay';
    overlay.innerHTML = '<div class="lightbox-backdrop"></div><div class="lightbox-container"><img src="' + escapeHTML(imageSrc) + '" alt="图片预览" class="lightbox-image"><button type="button" class="lightbox-close" title="关闭">&times;</button><button type="button" class="lightbox-download" title="下载"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg></button></div>';

    document.body.appendChild(overlay);

    requestAnimationFrame(() => {
      overlay.classList.add('lightbox-active');
    });

    const closeLightbox = () => {
      overlay.classList.remove('lightbox-active');
      setTimeout(() => {
        overlay.remove();
      }, 300);
    };

    overlay.querySelector('.lightbox-backdrop').addEventListener('click', closeLightbox);
    overlay.querySelector('.lightbox-close').addEventListener('click', closeLightbox);

    overlay.querySelector('.lightbox-download').addEventListener('click', () => {
      downloadImageSrc(imageSrc, options.filenamePrefix || 'image', options.format || 'png');
    });

    const handleEscape = (e) => {
      if (e.key === 'Escape') {
        closeLightbox();
        document.removeEventListener('keydown', handleEscape);
      }
    };
    document.addEventListener('keydown', handleEscape);
  }

  const HISTORY_KEY = 'image_gen_history';
  const HISTORY_LIMIT = 5;
  const HISTORY_IMAGE_DB = 'image_gen_history_images';
  const HISTORY_IMAGE_STORE = 'images';

  function openHistoryImageDB() {
    return new Promise((resolve, reject) => {
      if (!('indexedDB' in window)) {
        reject(new Error('indexedDB unavailable'));
        return;
      }
      const req = indexedDB.open(HISTORY_IMAGE_DB, 1);
      req.onupgradeneeded = () => {
        const db = req.result;
        if (!db.objectStoreNames.contains(HISTORY_IMAGE_STORE)) {
          db.createObjectStore(HISTORY_IMAGE_STORE);
        }
      };
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error || new Error('open indexedDB failed'));
    });
  }

  async function storeHistoryImage(id, imageSrc) {
    if (!imageSrc) return false;
    try {
      const db = await openHistoryImageDB();
      await new Promise((resolve, reject) => {
        const tx = db.transaction(HISTORY_IMAGE_STORE, 'readwrite');
        tx.objectStore(HISTORY_IMAGE_STORE).put(imageSrc, String(id));
        tx.oncomplete = resolve;
        tx.onerror = () => reject(tx.error || new Error('store image failed'));
      });
      db.close();
      return true;
    } catch (err) {
      console.warn('[历史记录] 原图保存失败:', err.message || err);
      return false;
    }
  }

  async function loadHistoryImage(id) {
    if (!id) return '';
    try {
      const db = await openHistoryImageDB();
      const imageSrc = await new Promise((resolve, reject) => {
        const tx = db.transaction(HISTORY_IMAGE_STORE, 'readonly');
        const req = tx.objectStore(HISTORY_IMAGE_STORE).get(String(id));
        req.onsuccess = () => resolve(req.result || '');
        req.onerror = () => reject(req.error || new Error('load image failed'));
      });
      db.close();
      return imageSrc || '';
    } catch (err) {
      console.warn('[历史记录] 原图读取失败:', err.message || err);
      return '';
    }
  }

  async function getHistoryImageData(entry) {
    if (!entry) return { src: '', isOriginal: false };
    const stored = await loadHistoryImage(entry.image_key || entry.id);
    if (stored) return { src: stored, isOriginal: true };
    if (entry.image) return { src: entry.image, isOriginal: true };
    if (entry.thumbnail) return { src: entry.thumbnail, isOriginal: false };
    return { src: '', isOriginal: false };
  }

  function useHistoryPrompt(prompt) {
    const textPanel = $('#panel-text');
    const textarea = textPanel?.querySelector('textarea.input, textarea.prompt-textarea');
    if (!textarea) {
      showToast('未找到文生图输入框', 'error');
      return;
    }
    textarea.value = prompt || '';
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
    const tab = $('.image-tab[data-mode="text"]');
    if (tab) tab.click();
    textarea.focus();
    showToast('已填入提示词', 'success');
  }

  async function publishHistoryEntry(entry, button) {
    const imageData = await getHistoryImageData(entry);
    if (!imageData.src) {
      showToast('这条记录没有可发布的图片数据', 'warning');
      return;
    }
    if (!imageData.isOriginal || !imageData.src.startsWith('data:image/')) {
      showToast('只有原图数据可发布到画廊', 'warning');
      return;
    }
    await publishGalleryItem(imageData.src, {
      prompt: entry.prompt || '',
      revisedPrompt: entry.revisedPrompt || entry.revised_prompt || '',
      model: entry.model || '',
      size: entry.size || entry.ratio || '',
      quality: entry.quality || '',
      outputFormat: entry.outputFormat || String(entry.format || 'png').toLowerCase(),
      mode: entry.generationMode || (entry.mode === '图生图' ? 'image' : 'text')
    }, button);
  }

  async function deleteHistoryImage(id) {
    if (!id) return;
    try {
      const db = await openHistoryImageDB();
      await new Promise((resolve, reject) => {
        const tx = db.transaction(HISTORY_IMAGE_STORE, 'readwrite');
        tx.objectStore(HISTORY_IMAGE_STORE).delete(String(id));
        tx.oncomplete = resolve;
        tx.onerror = () => reject(tx.error || new Error('delete image failed'));
      });
      db.close();
    } catch (err) {
      console.warn('[历史记录] 原图删除失败:', err.message || err);
    }
  }

  async function clearHistoryImages() {
    try {
      const db = await openHistoryImageDB();
      await new Promise((resolve, reject) => {
        const tx = db.transaction(HISTORY_IMAGE_STORE, 'readwrite');
        tx.objectStore(HISTORY_IMAGE_STORE).clear();
        tx.oncomplete = resolve;
        tx.onerror = () => reject(tx.error || new Error('clear images failed'));
      });
      db.close();
    } catch (err) {
      console.warn('[历史记录] 原图清空失败:', err.message || err);
    }
  }

  function getHistory() {
    try {
      const parsed = JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]');
      if (!Array.isArray(parsed)) return [];
      return parsed
        .filter(entry => entry && entry.id && entry.timestamp)
        .slice(0, HISTORY_LIMIT);
    } catch (e) {
      return [];
    }
  }

  function saveToHistory(entry) {
    const history = getHistory();
    history.unshift(entry);
    history.splice(HISTORY_LIMIT);

    let saved = false;

    while (!saved) {
      try {
        localStorage.setItem(HISTORY_KEY, JSON.stringify(history));
        saved = true;
      } catch (e) {
        if (e.name === 'QuotaExceededError' || e.code === 22) {
          if (history.length > 1) {
            history.pop();
            console.warn('[历史记录] 存储空间不足，已移除最旧记录，剩余 ' + history.length + ' 条');
          } else {
            console.error('[历史记录] 存储空间严重不足，无法保存');
            break;
          }
        } else {
          console.error('[历史记录] 存储失败:', e.message);
          break;
        }
      }
    }

    try {
      renderHistory();
    } catch (err) {
      console.warn('[历史记录] 刷新失败:', err.message || err);
    }
  }

  function clearHistory() {
    localStorage.removeItem(HISTORY_KEY);
    clearHistoryImages();
    try {
      renderHistory();
    } catch (err) {
      console.warn('[历史记录] 清空后刷新失败:', err.message || err);
    }
  }

  function ensureHistoryActionStyles() {
    if (document.getElementById('history-action-styles')) return;
    const style = document.createElement('style');
    style.id = 'history-action-styles';
    style.textContent = '.history-actions{display:flex;flex-wrap:wrap;gap:6px;margin-top:10px}.history-action{border:1px solid rgba(148,163,184,.45);background:rgba(255,255,255,.8);color:inherit;border-radius:6px;padding:4px 8px;font-size:12px;line-height:1.2;cursor:pointer;transition:background .15s,border-color .15s}.history-action:hover{background:rgba(59,130,246,.08);border-color:rgba(59,130,246,.45)}.history-action:disabled{opacity:.55;cursor:not-allowed}.history-delete.history-action{position:static;color:#dc2626}.history-thumb{appearance:none;-webkit-appearance:none;border:0;display:block;background-color:#e2e8f0;background-repeat:no-repeat}.history-thumb:focus-visible,.history-action:focus-visible,.history-prompt:focus-visible{outline:2px solid #60a5fa;outline-offset:2px}[data-theme="dark"] .history-action{background:rgba(15,23,42,.7);border-color:rgba(71,85,105,.8)}[data-theme="dark"] .history-action:hover{background:rgba(59,130,246,.18);border-color:rgba(96,165,250,.55)}';
    document.head.appendChild(style);
  }

  function renderHistory() {
    ensureHistoryActionStyles();
    const panel = $('#panel-history');
    if (!panel) return;
    const list = $('.history-list', panel);
    if (!list) return;

    const history = getHistory();

    if (history.length === 0) {
      list.innerHTML = '<div class="history-empty" style="grid-column:1/-1"><div class="image-empty-icon"><svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z"/></svg></div><h3>暂无记录</h3><p>生成图片后记录将显示在这里</p></div>';
      return;
    }

    list.innerHTML = history.map(entry => {
      const date = new Date(entry.timestamp);
      const isToday = new Date().toDateString() === date.toDateString();
      const timeLabel = isToday
        ? '今天 ' + String(date.getHours()).padStart(2, '0') + ':' + String(date.getMinutes()).padStart(2, '0')
        : (date.getMonth() + 1) + '月' + date.getDate() + '日 ' + String(date.getHours()).padStart(2, '0') + ':' + String(date.getMinutes()).padStart(2, '0');
      const thumbStyle = entry.thumbnail
        ? 'background-image:url(\'' + entry.thumbnail + '\');background-size:cover;background-position:center;cursor:pointer'
        : '';

      const promptText = entry.prompt || '未命名';
      const safePrompt = escapeHTML(promptText);
      return '<article class="history-card" data-id="' + entry.id + '" data-prompt="' + encodeURIComponent(promptText) + '" data-thumbnail="' + (entry.thumbnail || '') + '"><button type="button" class="history-thumb" style="' + thumbStyle + '" title="打开原图"></button><div class="history-info"><button type="button" class="history-prompt" title="填入文生图提示词">' + safePrompt + '</button><p>' + escapeHTML(entry.mode) + ' · ' + entry.count + ' 张 · ' + escapeHTML(entry.cost) + '</p><span>' + timeLabel + ' · ' + escapeHTML(entry.ratio) + ' · ' + escapeHTML(entry.format) + '</span><div class="history-actions"><button type="button" class="history-action history-use" data-id="' + entry.id + '">使用提示词</button><button type="button" class="history-action history-download" data-id="' + entry.id + '">下载原图</button><button type="button" class="history-action history-publish" data-id="' + entry.id + '">发布画廊</button><button type="button" class="history-action history-delete" data-id="' + entry.id + '">删除</button></div></div></article>';
    }).join('');

    $$('.history-thumb', list).forEach(thumb => {
      thumb.addEventListener('click', async e => {
        e.stopPropagation();
        const card = thumb.closest('.history-card');
        const id = parseInt(card?.dataset?.id || '0');
        const entry = getHistory().find(h => h.id === id);
        const imageData = await getHistoryImageData(entry);
        const imageSrc = imageData.src || card?.dataset?.thumbnail || '';
        if (imageSrc) {
          showImageLightbox(imageSrc, {
            filenamePrefix: 'history-image',
            format: entry?.outputFormat || entry?.format || 'png'
          });
          if (!imageData.isOriginal) showToast('当前只找到缩略图，原图可能已不可用', 'warning');
        } else {
          showToast('这条记录没有可打开的图片数据', 'warning');
        }
      });
    });

    $$('.history-prompt', list).forEach(btn => {
      btn.addEventListener('click', async e => {
        e.stopPropagation();
        const card = btn.closest('.history-card');
        const prompt = decodeURIComponent(card?.dataset?.prompt || '');
        if (!prompt) return;
        useHistoryPrompt(prompt);
      });
    });

    $$('.history-use', list).forEach(btn => {
      btn.addEventListener('click', e => {
        e.stopPropagation();
        const card = btn.closest('.history-card');
        const prompt = decodeURIComponent(card?.dataset?.prompt || '');
        if (prompt) useHistoryPrompt(prompt);
      });
    });

    $$('.history-download', list).forEach(btn => {
      btn.addEventListener('click', async e => {
        e.stopPropagation();
        const id = parseInt(btn.dataset.id);
        const entry = getHistory().find(h => h.id === id);
        const imageData = await getHistoryImageData(entry);
        if (!imageData.src) {
          showToast('这条记录没有可下载的图片数据', 'warning');
          return;
        }
        if (!imageData.isOriginal) {
          showToast('原图不可用，无法下载原图', 'warning');
          return;
        }
        downloadImageSrc(imageData.src, 'history-image', entry?.outputFormat || entry?.format || 'png');
      });
    });

    $$('.history-publish', list).forEach(btn => {
      btn.addEventListener('click', async e => {
        e.stopPropagation();
        const id = parseInt(btn.dataset.id);
        const entry = getHistory().find(h => h.id === id);
        if (!entry) {
          showToast('未找到这条历史记录', 'warning');
          return;
        }
        await publishHistoryEntry(entry, btn);
      });
    });

    $$('.history-delete', list).forEach(btn => {
      btn.addEventListener('click', e => {
        e.stopPropagation();
        const id = parseInt(btn.dataset.id);
        const entry = getHistory().find(h => h.id === id);
        const updated = getHistory().filter(h => h.id !== id);
        deleteHistoryImage(entry?.image_key || id);
        try {
          localStorage.setItem(HISTORY_KEY, JSON.stringify(updated));
        } catch (err) { /* ignore */ }
        try {
          renderHistory();
        } catch (err) {
          console.warn('[历史记录] 删除后刷新失败:', err.message || err);
        }
        showToast('记录已删除', 'info');
      });
    });
  }

  try {
    renderHistory();
  } catch (err) {
    console.warn('[历史记录] 初始化刷新失败:', err.message || err);
  }

  const historyPanel = $('#panel-history');
  if (historyPanel) {
    const head = $('.history-head', historyPanel);
    if (head && !head.querySelector('.history-clear-btn')) {
      const clearBtn = document.createElement('button');
      clearBtn.type = 'button';
      clearBtn.className = 'history-clear-btn';
      clearBtn.textContent = '清空记录';
      clearBtn.addEventListener('click', async () => {
        if (getHistory().length === 0) {
          showToast('暂无记录可清空', 'info');
          return;
        }
        const confirmed = await showConfirm('清空记录', '确认清空所有历史记录吗？清空后将无法恢复');
        if (confirmed) {
          clearHistory();
          showToast('记录已清空', 'success');
        }
      });
      head.appendChild(clearBtn);
    }
  }
})();
