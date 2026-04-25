/* ============================================
   HN — Frontend Application
   ============================================ */

// --- State ---
const state = {
  currentPage: 'top',
  storyIds: [],
  storyCache: new Map(),
  commentCache: new Map(),
  page: 1,
  pageSize: 20,
  loading: false,
  hasMore: true,
  currentStoryId: null,
};

// --- API Helpers ---
async function api(path, params = {}) {
  const url = new URL(`/api${path}`, location.origin);
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null) url.searchParams.set(k, v);
  }
  const res = await fetch(url);
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

// --- Time Formatting ---
function timeAgo(ts) {
  const seconds = Math.floor(Date.now() / 1000 - ts);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  return `${Math.floor(months / 12)}y ago`;
}

function formatDomain(url) {
  if (!url) return '';
  try {
    return new URL(url).hostname.replace('www.', '');
  } catch {
    return '';
  }
}

// --- Templates ---
function renderStoryCard(id, rank) {
  const story = state.storyCache.get(id);
  if (!story) return '';

  const tpl = document.getElementById('story-card-template');
  const clone = tpl.content.cloneNode(true);
  const card = clone.querySelector('.story-card');
  card.dataset.storyId = id;
  card.addEventListener('click', () => openStory(id));

  // Rank
  const rankEl = clone.querySelector('.story-rank');
  rankEl.textContent = `#${rank}`;

  // Domain
  const domain = formatDomain(story.url);
  const domainEl = clone.querySelector('.story-domain');
  if (story.url) {
    domainEl.textContent = domain;
    domainEl.addEventListener('click', (e) => {
      e.stopPropagation();
      window.open(story.url, '_blank');
    });
  } else {
    domainEl.style.display = 'none';
    // Hide adjacent separator
    const sep = domainEl.nextElementSibling;
    if (sep && sep.classList.contains('story-meta-sep')) sep.style.display = 'none';
  }

  // Meta
  clone.querySelector('.story-age').textContent = timeAgo(story.time);
  clone.querySelector('.story-author').textContent = story.by || '';

  // Title
  clone.querySelector('.story-title').textContent = story.title || '(No title)';

  // Stats
  clone.querySelector('.stat-points').textContent = story.score || 0;
  clone.querySelector('.stat-comments').textContent = story.descendants || 0;

  // URL button
  const urlBtn = clone.querySelector('.story-url-btn');
  if (story.url) {
    urlBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      window.open(story.url, '_blank');
    });
  } else {
    urlBtn.style.display = 'none';
  }

  return clone;
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

function renderComment(item) {
  if (!item || !item.id) return null;
  const tpl = document.getElementById('comment-template');
  const clone = tpl.content.cloneNode(true);

  const commentEl = clone.querySelector('.comment');
  commentEl.id = `comment-${item.id}`;

  clone.querySelector('.comment-author').textContent = item.author || 'deleted';
  clone.querySelector('.comment-time').textContent = timeAgo(item.time);

  const bodyEl = clone.querySelector('.comment-body');
  bodyEl.innerHTML = item.content || '';
  bodyEl.addEventListener('click', () => {
    const children = commentEl.querySelector('.comment-children');
    if (children) {
      commentEl.classList.toggle('collapsed');
      children.hidden = commentEl.classList.contains('collapsed');
    }
  });

  const childrenContainer = clone.querySelector('.comment-children');
  if (item.children && item.children.length > 0) {
    const childrenFragment = document.createDocumentFragment();
    for (const child of item.children) {
      const childEl = renderComment(child);
      if (childEl) childrenFragment.appendChild(childEl);
    }
    childrenContainer.appendChild(childrenFragment);
    childrenContainer.hidden = false;
  } else {
    childrenContainer.remove();
  }

  return commentEl;
}

// --- Loading ---
function showLoading() {
  const el = document.getElementById('loadingOverlay');
  if (el) el.classList.add('visible');
}

function hideLoading() {
  const el = document.getElementById('loadingOverlay');
  if (el) el.classList.remove('visible');
}

// --- Navigation ---
async function navigate(page) {
  if (state.loading) return;

  // Update nav buttons
  document.querySelectorAll('.nav-btn').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.page === page);
  });

  // Clear state for new page
  state.currentPage = page;
  state.storyIds = [];
  state.storyCache.clear();
  state.page = 1;
  state.hasMore = true;
  state.currentStoryId = null;

  // Show loading and feed view
  showLoading();

  // Show feed view
  showFeedView();
  await loadStories();
}

function showFeedView() {
  const main = document.getElementById('mainContent');
  const tpl = document.getElementById('story-list-template');
  main.innerHTML = tpl.innerHTML;
  main.classList.add('page-enter');

  const titles = {
    top: 'Top Stories',
    new: 'New Stories',
    show: 'Show HN',
    ask: 'Ask HN',
    jobs: 'Jobs',
  };
  document.getElementById('feedTitle').textContent = titles[state.currentPage] || 'Stories';

  // Show footer on feed view
  document.getElementById('footer').style.display = '';
}

function showDetailView() {
  const main = document.getElementById('mainContent');
  const tpl = document.getElementById('story-detail-template');
  main.innerHTML = tpl.innerHTML;
  main.classList.add('page-enter');

  // Hide footer on detail view
  document.getElementById('footer').style.display = 'none';
}

function showSearchResults(query) {
  const main = document.getElementById('mainContent');
  const tpl = document.getElementById('search-results-template');
  main.innerHTML = tpl.innerHTML;
  main.classList.add('page-enter');
  document.getElementById('searchSubtitle').textContent = `Results for "${escapeHtml(query)}"`;
}

// --- Loading Stories ---
async function loadStories() {
  if (state.loading || !state.hasMore) return;
  state.loading = true;

  const storyList = document.getElementById('storyList');
  const loadMoreWrap = document.getElementById('loadMoreWrap');

  try {
    const data = await api('/stories', {
      type: state.currentPage,
      page: state.page,
      limit: state.pageSize,
    });

    if (data.ids.length === 0) {
      state.hasMore = false;
      if (loadMoreWrap) loadMoreWrap.hidden = true;
      return;
    }

    // Cache stories
    for (const story of data.stories) {
      state.storyCache.set(story.id, story);
    }

    // Render cards
    const startRank = (state.page - 1) * state.pageSize + 1;
    const fragment = document.createDocumentFragment();
    data.ids.forEach((id, i) => {
      const card = renderStoryCard(id, startRank + i);
      if (card) {
        if (card.nodeType) {
          // DocumentFragment from template
          const el = card.querySelector('.story-card') || card.firstChild;
          if (el) {
            el.style.animationDelay = `${i * 30}ms`;
            fragment.appendChild(card);
          }
        } else {
          fragment.appendChild(new DOMParser().parseFromString(card, 'text/html').body.firstChild);
        }
      }
    });

    if (storyList) {
      storyList.appendChild(fragment);
    }

    state.storyIds = [...state.storyIds, ...data.ids];
    state.page++;
    state.hasMore = data.hasMore;

    if (loadMoreWrap) {
      loadMoreWrap.hidden = !data.hasMore;
    }

    // Update subtitle
    const subtitle = document.getElementById('feedSubtitle');
    if (subtitle) {
      const now = new Date();
      const timeStr = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      subtitle.textContent = `Showing ${state.storyIds.length} stories · Updated ${timeStr}`;
    }

  } catch (err) {
    console.error('Failed to load stories:', err);
    if (storyList && storyList.children.length === 0) {
      storyList.innerHTML = `<div class="empty-state">
        <div class="empty-state-icon">⚠</div>
        <div class="empty-state-text">Failed to load stories. Please try again.</div>
      </div>`;
    }
  } finally {
    state.loading = false;
    hideLoading();
  }
}

async function loadMore() {
  await loadStories();
}

// --- Search ---
async function handleSearchKey(event) {
  if (event.key === 'Enter') {
    const query = event.target.value.trim();
    if (!query) return;
    await performSearch(query);
  }
}

async function performSearch(query) {
  state.loading = true;
  state.storyCache.clear();
  state.storyIds = [];
  state.page = 1;

  showLoading();
  const main = document.getElementById('mainContent');
  showSearchResults(query);

  try {
    const data = await api('/search', { q: query });

    for (const story of data.stories) {
      state.storyCache.set(story.id, story);
    }

    const storyList = document.getElementById('storyList');
    if (storyList && data.stories.length > 0) {
      const fragment = document.createDocumentFragment();
      data.stories.forEach((story, i) => {
        const card = renderStoryCard(story.id, i + 1);
        if (card && card.nodeType) {
          const el = card.querySelector('.story-card');
          if (el) {
            el.style.animationDelay = `${i * 30}ms`;
            fragment.appendChild(card);
          }
        }
      });
      storyList.innerHTML = '';
      storyList.appendChild(fragment);
    } else if (storyList) {
      storyList.innerHTML = `<div class="empty-state">
        <div class="empty-state-icon">🔍</div>
        <div class="empty-state-text">No stories found for "${escapeHtml(query)}"</div>
      </div>`;
    }
  } catch (err) {
    console.error('Search failed:', err);
  } finally {
    state.loading = false;
    hideLoading();
  }
}

// --- Story Detail ---
async function openStory(id) {
  if (state.loading) return;
  state.currentStoryId = id;
  showDetailView();

  const story = state.storyCache.get(id);
  if (!story) {
    try {
      const data = await api(`/story/${id}`);
      state.storyCache.set(id, data);
      renderDetailArticle(data);
    } catch (err) {
      console.error('Failed to load story:', err);
      return;
    }
  } else {
    renderDetailArticle(story);
  }

  // Load comments
  loadComments(id);
}

function renderDetailArticle(story) {
  const article = document.getElementById('detailArticle');
  const domain = formatDomain(story.url);

  let urlBlock = '';
  if (story.url) {
    urlBlock = `<a href="${escapeHtml(story.url)}" class="detail-url" target="_blank" rel="noopener">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg>
      ${escapeHtml(story.url)}
    </a>`;
  }

  article.innerHTML = `
    ${domain ? `<a href="${escapeHtml(story.url || '#')}" class="detail-domain" target="_blank" rel="noopener">${escapeHtml(domain)}</a>` : ''}
    <h1 class="detail-title">${escapeHtml(story.title)}</h1>
    <div class="detail-meta">
      <span class="detail-meta-item">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
        ${escapeHtml(story.by || 'Unknown')}
      </span>
      <span class="detail-meta-item">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
        ${timeAgo(story.time)}
      </span>
      <span class="detail-meta-item">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M7 10v12"/><path d="M15 5.88 14 12h5.93l-1.05 6.39-8.89 5.24-8.89-5.24L5.07 12H11l-1.06-6.12"/><path d="m15 5.88 1-6.12"/></svg>
        ${story.score || 0} points
      </span>
      <span class="detail-meta-item">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
        ${story.descendants || 0} comments
      </span>
    </div>
    ${urlBlock}
  `;
}

async function loadComments(id) {
  const commentsList = document.getElementById('commentsList');
  const commentsTitle = document.getElementById('commentsTitle');

  try {
    const data = await api(`/comments/${id}`);

    if (data.comments && data.comments.length > 0) {
      commentsTitle.innerHTML = `Discussion <span class="count">(${data.comments.length} comments)</span>`;
      const fragment = document.createDocumentFragment();
      for (const comment of data.comments) {
        const el = renderComment(comment);
        if (el) fragment.appendChild(el);
      }
      commentsList.innerHTML = '';
      commentsList.appendChild(fragment);
    } else {
      commentsTitle.textContent = 'Discussion';
      commentsList.innerHTML = `<div class="empty-state">
        <div class="empty-state-icon">💬</div>
        <div class="empty-state-text">No comments yet</div>
      </div>`;
    }
  } catch (err) {
    console.error('Failed to load comments:', err);
    commentsList.innerHTML = `<div class="empty-state">
      <div class="empty-state-icon">⚠</div>
      <div class="empty-state-text">Failed to load comments</div>
    </div>`;
  }
}

function closeStory() {
  state.currentStoryId = null;
  navigate(state.currentPage);
}

// --- Keyboard Shortcuts ---
document.addEventListener('keydown', (e) => {
  // Escape to go back
  if (e.key === 'Escape') {
    if (state.currentStoryId) {
      closeStory();
    } else {
      // Focus search
      document.getElementById('searchInput').focus();
    }
  }

  // '/' to focus search
  if (e.key === '/' && !state.currentStoryId) {
    e.preventDefault();
    document.getElementById('searchInput').focus();
  }

  // 'n' to refresh current feed (when not in search input)
  if (e.key === 'n' && !state.currentStoryId && document.activeElement.tagName !== 'INPUT') {
    e.preventDefault();
    navigate(state.currentPage);
  }
});

// --- Infinite Scroll ---
let scrollTimeout;
window.addEventListener('scroll', () => {
  clearTimeout(scrollTimeout);
  scrollTimeout = setTimeout(() => {
    if (state.currentStoryId) return;
    if (state.loading || !state.hasMore) return;

    const scrollY = window.scrollY || window.pageYOffset;
    const windowHeight = window.innerHeight;
    const docHeight = document.documentElement.scrollHeight;

    if (scrollY + windowHeight >= docHeight - 400) {
      loadMore();
    }
  }, 100);
});

// --- Init ---
async function init() {
  await navigate('top');
}

init();
