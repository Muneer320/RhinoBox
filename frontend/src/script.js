// Import API functions
import {
  ingestFiles,
  ingestMedia,
  ingestJSON,
  getFiles,
  getFile,
  deleteFile,
  renameFile,
  getNotes,
  addNote,
  deleteNote,
  getStatistics,
  getCollections,
  getCollectionStats,
  searchFiles,
  API_CONFIG,
  APIError,
} from "./api.js";

const root = document.documentElement;
const THEME_KEY = "rhinobox-theme";
let currentCollectionType = null;
let modeToggle = null;
let toast = null;

// Initialize dropzone and form when DOM is ready
function initHomePageFeatures() {
  const dropzone = document.getElementById("dropzone");
  const fileInput = document.getElementById("fileInput");
  const quickAddTrigger = document.getElementById("quickAddTrigger");
  const quickAddForm = document.getElementById("quickAddForm");
  const quickAddClose = document.getElementById("quickAdd-close-button");
  const quickAddCancel = document.getElementById("quickAdd-cancel");

  if (!dropzone || !fileInput) {
    // Elements not found, try again after a short delay
    setTimeout(initHomePageFeatures, 100);
    return;
  }

  // Setup dropzone click
  dropzone.addEventListener("click", () => {
    fileInput.click();
  });

  // Setup dropzone keyboard navigation
  dropzone.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      fileInput.click();
    }
  });

  // Setup drag and drop
  dropzone.addEventListener("dragover", (event) => {
    event.preventDefault();
    event.stopPropagation();
    dropzone.classList.add("is-active");
  });

  dropzone.addEventListener("dragenter", (event) => {
    event.preventDefault();
    event.stopPropagation();
    dropzone.classList.add("is-active");
  });

  dropzone.addEventListener("dragleave", (event) => {
    event.preventDefault();
    event.stopPropagation();
    // Only remove active class if we're leaving the dropzone itself
    if (!dropzone.contains(event.relatedTarget)) {
      dropzone.classList.remove("is-active");
    }
  });

  dropzone.addEventListener("drop", async (event) => {
    event.preventDefault();
    event.stopPropagation();
    dropzone.classList.remove("is-active");

    const files = Array.from(event.dataTransfer.files || []);
    if (files.length > 0) {
      await uploadFiles(files);
    } else {
      showToast("Drop recognized, but no files detected");
    }
  });

  // Setup file input change
  fileInput.addEventListener("change", async () => {
    const files = Array.from(fileInput.files || []);
    if (files.length > 0) {
      await uploadFiles(files);
    }
    fileInput.value = "";
  });

  // Setup upload button (New button) - only for file uploads
  const uploadButton = document.getElementById("uploadButton");
  if (uploadButton) {
    uploadButton.addEventListener("click", () => {
      fileInput.click();
    });
  }

  // Setup quick add panel trigger (separate tab button)
  const quickAddPanel = document.getElementById("quickAdd-panel");
  const quickAddOverlay = document.getElementById("quickAdd-overlay");
  
  if (quickAddTrigger && quickAddPanel) {
    quickAddTrigger.addEventListener("click", () => {
      quickAddPanel.classList.add("is-open");
      document.body.style.overflow = "hidden";
      const textarea = document.getElementById("quickAdd");
      if (textarea) {
        setTimeout(() => textarea.focus(), 100);
      }
    });

    // Close panel handlers
    const closePanel = () => {
      quickAddPanel.classList.remove("is-open");
      document.body.style.overflow = "";
      const textarea = document.getElementById("quickAdd");
      const typeSelect = document.getElementById("quickAddType");
      if (textarea) textarea.value = "";
      if (typeSelect) typeSelect.value = "text";
    };

    if (quickAddClose) {
      quickAddClose.addEventListener("click", closePanel);
    }
    if (quickAddCancel) {
      quickAddCancel.addEventListener("click", closePanel);
    }

    // Close on overlay click
    if (quickAddOverlay) {
      quickAddOverlay.addEventListener("click", closePanel);
    }

    // Close on Escape key
    document.addEventListener("keydown", (e) => {
      if (e.key === "Escape" && quickAddPanel.classList.contains("is-open")) {
        closePanel();
      }
    });
  }

  // Setup quick add form
  if (quickAddForm) {
    quickAddForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const textarea = document.getElementById("quickAdd");
      const typeSelect = document.getElementById("quickAddType");
      const value = textarea?.value.trim() || "";
      const selectedType = typeSelect?.value || "text";

      if (!value) {
        showToast("Provide a link, query, or description first", "warning");
        if (textarea) textarea.focus();
        return;
      }

      try {
        let documents = [];
        
        // Handle different types
        if (selectedType === "url") {
          // Treat as URL
          documents = [{ content: value, type: "url", url: value }];
        } else if (selectedType === "json") {
          // Try to parse as JSON
          try {
            const parsed = JSON.parse(value);
            documents = Array.isArray(parsed) ? parsed : [parsed];
          } catch {
            showToast("Invalid JSON format", "error");
            return;
          }
        } else {
          // Other types (text, python, javascript, etc.)
          documents = [{ content: value, type: selectedType }];
        }

        const processingToast = showToast("Processing...", "info", 0);
        await ingestJSON(documents, "quick-add", `Quick add: ${selectedType}`);
        dismissToast(processingToast);
        showToast("Successfully added item", "success");
        
        // Close panel and reset
        const quickAddPanel = document.getElementById("quickAdd-panel");
        if (quickAddPanel) {
          quickAddPanel.classList.remove("is-open");
          document.body.style.overflow = "";
        }
        if (textarea) textarea.value = "";
        if (typeSelect) typeSelect.value = "text";

        // Reload current collection if viewing one
        if (currentCollectionType) {
          loadCollectionFiles(currentCollectionType);
        }
      } catch (error) {
        console.error("Quick add error:", error);
        let errorMessage = "Unknown error";
        if (error instanceof APIError) {
          errorMessage = error.getUserMessage();
        } else if (error.message) {
          errorMessage = error.message;
        }
        showToast(`Failed to add item: ${errorMessage}`, "error");
      }
    });
  }
}

function applyTheme(theme) {
  root.setAttribute("data-theme", theme);
  const isDark = theme === "dark";
  if (modeToggle) {
    modeToggle.innerHTML = isDark
      ? '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="0.75" stroke-linecap="round" stroke-linejoin="round"><path d="M14.828 14.828a4 4 0 1 0 -5.656 -5.656a4 4 0 0 0 5.656 5.656z" /><path d="M6.343 17.657l-1.414 1.414" /><path d="M6.343 6.343l-1.414 -1.414" /><path d="M17.657 6.343l1.414 -1.414" /><path d="M17.657 17.657l1.414 1.414" /><path d="M4 12h-2" /><path d="M12 4v-2" /><path d="M20 12h2" /><path d="M12 20v2" /></svg>'
      : '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M21.752 15.002A9.72 9.72 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z" /></svg>';
  }
  // Update NoSQL diagram colors when theme changes
  updateNosqlDiagramColors();
}

// NoSQL Zoom and Pan functionality
let nosqlZoomPanInitialized = false;
let nosqlZoom = 1;
let nosqlPanX = 0;
let nosqlPanY = 0;
let isPanning = false;
let startPanX = 0;
let startPanY = 0;

function initNosqlZoomPan() {
  const nosqlSvg = document.getElementById("nosql-svg");
  const nosqlDiagram = document.getElementById("nosql-diagram");
  const zoomInBtn = document.getElementById("nosql-zoom-in");
  const zoomOutBtn = document.getElementById("nosql-zoom-out");
  const resetBtn = document.getElementById("nosql-reset");

  if (!nosqlSvg || !nosqlDiagram) return;
  
  // Reset initialization flag if elements are recreated
  if (!nosqlSvg.querySelector("g.nosql-transform-group")) {
    nosqlZoomPanInitialized = false;
  }
  
  if (nosqlZoomPanInitialized) return;
  nosqlZoomPanInitialized = true;

  // Reset zoom and pan
  nosqlZoom = 1;
  nosqlPanX = 0;
  nosqlPanY = 0;
  
  // Initialize transform group and apply initial transform
  setTimeout(() => {
    updateNosqlTransform();
  }, 50);

  // Zoom in button
  if (zoomInBtn) {
    zoomInBtn.addEventListener("click", () => {
      nosqlZoom = Math.min(nosqlZoom * 1.2, 5);
      updateNosqlTransform();
    });
  }

  // Zoom out button
  if (zoomOutBtn) {
    zoomOutBtn.addEventListener("click", () => {
      nosqlZoom = Math.max(nosqlZoom / 1.2, 0.5);
      updateNosqlTransform();
    });
  }

  // Reset button
  if (resetBtn) {
    resetBtn.addEventListener("click", () => {
      nosqlZoom = 1;
      nosqlPanX = 0;
      nosqlPanY = 0;
      updateNosqlTransform();
    });
  }

  // Mouse wheel zoom
  nosqlDiagram.addEventListener("wheel", (e) => {
    e.preventDefault();
    const delta = e.deltaY > 0 ? 0.9 : 1.1;
    nosqlZoom = Math.max(0.5, Math.min(5, nosqlZoom * delta));
    updateNosqlTransform();
  });

  // Mouse drag pan
  nosqlDiagram.addEventListener("mousedown", (e) => {
    if (e.button === 0) {
      isPanning = true;
      startPanX = e.clientX - nosqlPanX;
      startPanY = e.clientY - nosqlPanY;
      nosqlDiagram.style.cursor = "grabbing";
    }
  });

  document.addEventListener("mousemove", (e) => {
    if (isPanning) {
      nosqlPanX = e.clientX - startPanX;
      nosqlPanY = e.clientY - startPanY;
      updateNosqlTransform();
    }
  });

  document.addEventListener("mouseup", () => {
    if (isPanning) {
      isPanning = false;
      if (nosqlDiagram) nosqlDiagram.style.cursor = "grab";
    }
  });

  // Touch support for mobile
  let touchStartDistance = 0;
  let touchStartZoom = 1;
  let touchStartPanX = 0;
  let touchStartPanY = 0;
  let touchStartX = 0;
  let touchStartY = 0;

  nosqlDiagram.addEventListener("touchstart", (e) => {
    if (e.touches.length === 1) {
      isPanning = true;
      touchStartX = e.touches[0].clientX;
      touchStartY = e.touches[0].clientY;
      touchStartPanX = nosqlPanX;
      touchStartPanY = nosqlPanY;
    } else if (e.touches.length === 2) {
      isPanning = false;
      const dx = e.touches[0].clientX - e.touches[1].clientX;
      const dy = e.touches[0].clientY - e.touches[1].clientY;
      touchStartDistance = Math.sqrt(dx * dx + dy * dy);
      touchStartZoom = nosqlZoom;
    }
  });

  nosqlDiagram.addEventListener("touchmove", (e) => {
    e.preventDefault();
    if (e.touches.length === 1 && isPanning) {
      nosqlPanX = touchStartPanX + (e.touches[0].clientX - touchStartX);
      nosqlPanY = touchStartPanY + (e.touches[0].clientY - touchStartY);
      updateNosqlTransform();
    } else if (e.touches.length === 2) {
      const dx = e.touches[0].clientX - e.touches[1].clientX;
      const dy = e.touches[0].clientY - e.touches[1].clientY;
      const distance = Math.sqrt(dx * dx + dy * dy);
      nosqlZoom = Math.max(0.5, Math.min(5, touchStartZoom * (distance / touchStartDistance)));
      updateNosqlTransform();
    }
  });

  nosqlDiagram.addEventListener("touchend", () => {
    isPanning = false;
  });

  // Set initial cursor
  nosqlDiagram.style.cursor = "grab";
}

function updateNosqlTransform() {
  const nosqlSvg = document.getElementById("nosql-svg");
  if (!nosqlSvg) return;

  let g = nosqlSvg.querySelector("g.nosql-transform-group");
  
  // Create transform group if it doesn't exist
  if (!g) {
    g = document.createElementNS("http://www.w3.org/2000/svg", "g");
    g.classList.add("nosql-transform-group");
    // Move all children to the transform group (except defs which should stay at root)
    const children = Array.from(nosqlSvg.childNodes);
    children.forEach(child => {
      if (child.nodeType === Node.ELEMENT_NODE && child.tagName !== "defs") {
        g.appendChild(child);
      }
    });
    // Insert after defs
    const defs = nosqlSvg.querySelector("defs");
    if (defs) {
      nosqlSvg.insertBefore(g, defs.nextSibling);
    } else {
      nosqlSvg.appendChild(g);
    }
  }

  // Apply transform: translate to center, scale, translate back, then pan
  const centerX = 600; // viewBox center X (half of 1200)
  const centerY = 400; // viewBox center Y (half of 800)

  g.setAttribute(
    "transform",
    `translate(${centerX + nosqlPanX}, ${centerY + nosqlPanY}) scale(${nosqlZoom}) translate(${-centerX}, ${-centerY})`
  );
}

// Update NoSQL diagram SVG colors based on current theme
function updateNosqlDiagramColors() {
  const nosqlSvg = document.querySelector(".nosql-svg");
  if (!nosqlSvg) return;

  const computedStyle = getComputedStyle(document.documentElement);
  const surface = computedStyle.getPropertyValue("--surface").trim();
  const surfaceMuted = computedStyle.getPropertyValue("--surface-muted").trim();
  const border = computedStyle.getPropertyValue("--border").trim();
  const borderStrong = computedStyle.getPropertyValue("--border-strong").trim();
  const accent = computedStyle.getPropertyValue("--accent").trim();
  const textPrimary = computedStyle.getPropertyValue("--text-primary").trim();
  const textSecondary = computedStyle.getPropertyValue("--text-secondary").trim();

  // Update all collection box rectangles
  nosqlSvg.querySelectorAll(".collection-box rect").forEach((rect, index) => {
    // First rect is the main box, second is the header
    if (index % 2 === 0) {
      rect.setAttribute("fill", surface);
      rect.setAttribute("stroke", border);
    } else {
      rect.setAttribute("fill", accent);
    }
  });

  // Update embedded box rectangles
  nosqlSvg.querySelectorAll(".embedded-box rect").forEach((rect) => {
    rect.setAttribute("fill", surfaceMuted);
    rect.setAttribute("stroke", border);
  });

  // Update collection box text - first text in each box is the title (white), rest are fields
  nosqlSvg.querySelectorAll(".collection-box").forEach((box) => {
    const texts = box.querySelectorAll("text");
    texts.forEach((text, index) => {
      if (index === 0) {
        text.setAttribute("fill", "white");
      } else {
        text.setAttribute("fill", textPrimary);
      }
    });
  });

  // Update embedded box text - first text is the title (secondary), rest are fields
  nosqlSvg.querySelectorAll(".embedded-box").forEach((box) => {
    const texts = box.querySelectorAll("text");
    texts.forEach((text, index) => {
      if (index === 0) {
        text.setAttribute("fill", textSecondary);
      } else {
        text.setAttribute("fill", textPrimary);
      }
    });
  });

  // Update relationship lines
  nosqlSvg.querySelectorAll("line").forEach((line) => {
    const strokeAttr = line.getAttribute("stroke");
    if (strokeAttr && strokeAttr.includes("border-strong")) {
      line.setAttribute("stroke", borderStrong);
    } else if (strokeAttr && strokeAttr.includes("accent")) {
      line.setAttribute("stroke", accent);
    }
  });

  // Update arrow marker
  const marker = nosqlSvg.querySelector("marker#arrowhead polygon");
  if (marker) {
    marker.setAttribute("fill", accent);
  }
}

function getStoredTheme() {
  return localStorage.getItem(THEME_KEY);
}

function initTheme() {
  const stored = getStoredTheme();
  const prefersDark = window.matchMedia("(prefers-color-scheme: dark)");
  if (stored) {
    applyTheme(stored);
    return;
  }
  applyTheme(prefersDark.matches ? "dark" : "light");
}

// Initialize theme toggle
let themeToggleInitialized = false;
function initThemeToggle() {
  if (themeToggleInitialized) return; // Prevent multiple initializations

  modeToggle = document.getElementById("modeToggle");
  if (!modeToggle) {
    // Button not found, try again (with max retries)
    if (typeof initThemeToggle.retryCount === "undefined") {
      initThemeToggle.retryCount = 0;
    }
    if (initThemeToggle.retryCount < 10) {
      initThemeToggle.retryCount++;
      setTimeout(initThemeToggle, 100);
    } else {
      console.error("Mode toggle button not found after 10 retries");
    }
    return;
  }

  themeToggleInitialized = true;

  // Add click event listener (only once)
  if (!modeToggle.hasAttribute("data-listener-attached")) {
    modeToggle.setAttribute("data-listener-attached", "true");
    modeToggle.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      const current = root.getAttribute("data-theme") || "light";
      const next = current === "dark" ? "light" : "dark";
      applyTheme(next);
      localStorage.setItem(THEME_KEY, next);
      showToast(`Switched to ${next} mode`);
    });
  }

  // Listen for system theme changes (only once)
  if (!window.prefersDarkListenerAdded) {
    window.prefersDarkListenerAdded = true;
    const prefersDark = window.matchMedia("(prefers-color-scheme: dark)");
    prefersDark.addEventListener("change", (event) => {
      if (!getStoredTheme()) {
        applyTheme(event.matches ? "dark" : "light");
        showToast(
          `System theme changed to ${event.matches ? "dark" : "light"}`
        );
      }
    });
  }
}

// Page navigation
function showPage(pageId) {
  const allPages = document.querySelectorAll(".page-content");
  allPages.forEach((page) => {
    page.style.display = "none";
  });
  const targetPage = document.getElementById(`page-${pageId}`);
  if (targetPage) {
    targetPage.style.display = "flex";
  }
}

// Initialize sidebar navigation
function initSidebarNavigation() {
  const sidebarButtons = document.querySelectorAll(".sidebar-button");

  if (sidebarButtons.length === 0) {
    // Buttons not found yet, try again
    setTimeout(initSidebarNavigation, 100);
    return;
  }

  sidebarButtons.forEach((button) => {
    button.addEventListener("click", async () => {
      const target = button.dataset.target;
      if (!target) {
        console.warn("Sidebar button missing data-target attribute");
        return;
      }

      // Remove active class from all buttons
      sidebarButtons.forEach((btn) => btn.classList.remove("is-active"));
      // Add active class to clicked button
      button.classList.add("is-active");

      // Show the target page
      showPage(target);

      // Load data when switching pages
      if (target === "statistics") {
        await loadStatistics();
      } else if (target === "files") {
        // Load collections when switching to files page
        await loadCollections();
      } else if (target === "data") {
        // Initialize data tabs when switching to data page
        initDataTabs();
        // Update diagram colors
        setTimeout(updateNosqlDiagramColors, 100);
      }

      showToast(
        `Switched to ${
          target === "home"
            ? "Home"
            : target.charAt(0).toUpperCase() + target.slice(1)
        }`
      );
    });
  });
}

// ==================== Global Search Functionality ====================

// Debounce function to limit API calls
function debounce(func, delay) {
  let timeoutId;
  return function (...args) {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => func.apply(this, args), delay);
  };
}

// Search state
let searchModal = null;
let searchInput = null;
let searchResultsList = null;
let searchLoading = null;
let searchEmpty = null;
let searchQueryDisplay = null;
let searchCloseButton = null;
let selectedSearchIndex = -1;
let currentSearchResults = [];

// Initialize global search
function initGlobalSearch() {
  searchInput = document.getElementById("global-search");
  searchModal = document.getElementById("search-modal");
  searchResultsList = document.getElementById("search-results-list");
  searchLoading = document.getElementById("search-loading");
  searchEmpty = document.getElementById("search-empty");
  searchQueryDisplay = document.getElementById("search-query-display");
  searchCloseButton = document.getElementById("search-close-button");

  if (!searchInput || !searchModal) {
    setTimeout(initGlobalSearch, 100);
    return;
  }

  // Debounced search handler
  const debouncedSearch = debounce(async (query) => {
    if (!query || query.trim().length < 2) {
      closeSearchModal();
      return;
    }

    await performSearch(query.trim());
  }, 500);

  // Input event listener
  searchInput.addEventListener("input", (e) => {
    const query = e.target.value;
    debouncedSearch(query);
  });

  // Enter key to open results or select
  searchInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      const query = searchInput.value.trim();
      if (query.length >= 2) {
        // If modal is open and there's a selection, navigate to it
        if (
          searchModal.style.display !== "none" &&
          selectedSearchIndex >= 0 &&
          currentSearchResults[selectedSearchIndex]
        ) {
          navigateToSearchResult(currentSearchResults[selectedSearchIndex]);
        } else {
          // Otherwise trigger search immediately
          performSearch(query);
        }
      }
    } else if (e.key === "Escape") {
      closeSearchModal();
      searchInput.blur();
    }
  });

  // Close button
  if (searchCloseButton) {
    searchCloseButton.addEventListener("click", closeSearchModal);
  }

  // Close on overlay click
  if (searchModal) {
    const overlay = searchModal.querySelector(".comments-modal-overlay");
    if (overlay) {
      overlay.addEventListener("click", closeSearchModal);
    }
  }

  // Keyboard navigation in search results
  document.addEventListener("keydown", (e) => {
    if (searchModal && searchModal.style.display !== "none") {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        navigateSearchResults(1);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        navigateSearchResults(-1);
      } else if (e.key === "Enter" && selectedSearchIndex >= 0) {
        e.preventDefault();
        if (currentSearchResults[selectedSearchIndex]) {
          navigateToSearchResult(currentSearchResults[selectedSearchIndex]);
        }
      }
    }
  });
}

// Perform search API call
async function performSearch(query) {
  if (!searchModal || !searchResultsList) return;

  try {
    // Show modal and loading state
    searchModal.style.display = "flex";
    searchResultsList.innerHTML = "";
    if (searchLoading) searchLoading.style.display = "flex";
    if (searchEmpty) searchEmpty.style.display = "none";
    if (searchQueryDisplay)
      searchQueryDisplay.textContent = `Searching for "${query}"...`;

    selectedSearchIndex = -1;
    currentSearchResults = [];

    // Call search API
    const response = await searchFiles(query);

    // Hide loading
    if (searchLoading) searchLoading.style.display = "none";

    if (searchQueryDisplay) {
      searchQueryDisplay.textContent = `${response.count || 0} result${
        response.count !== 1 ? "s" : ""
      } for "${query}"`;
    }

    if (!response.results || response.results.length === 0) {
      if (searchEmpty) searchEmpty.style.display = "flex";
      return;
    }

    // Store results for keyboard navigation
    currentSearchResults = response.results;

    // Display results
    searchResultsList.innerHTML = response.results
      .map((file, index) => {
        const icon = getFileIcon(file.type || file.original_name);
        const size = formatFileSize(file.size || 0);
        const date = file.modified_at || file.ingested_at || "";
        const formattedDate = date
          ? new Date(date).toLocaleDateString()
          : "Unknown date";

        return `
        <div class="search-result-item" data-index="${index}" data-file-id="${
          file.id || file.hash
        }" tabindex="0">
          <div class="search-result-icon">
            ${icon}
          </div>
          <div class="search-result-info">
            <p class="search-result-name">${escapeHtml(
              file.original_name || file.name || "Unnamed file"
            )}</p>
            <p class="search-result-details">${size} • ${formattedDate} • ${escapeHtml(
          file.type || "Unknown type"
        )}</p>
          </div>
        </div>
      `;
      })
      .join("");

    // Add click handlers to results
    const resultItems = searchResultsList.querySelectorAll(
      ".search-result-item"
    );
    resultItems.forEach((item, index) => {
      item.addEventListener("click", () => {
        navigateToSearchResult(currentSearchResults[index]);
      });

      item.addEventListener("mouseenter", () => {
        selectedSearchIndex = index;
        updateSearchSelection();
      });
    });
  } catch (error) {
    console.error("Search error:", error);
    if (searchLoading) searchLoading.style.display = "none";
    if (searchEmpty) searchEmpty.style.display = "flex";
    if (searchResultsList) {
      let errorMessage = "Error performing search. Please try again.";
      if (error instanceof APIError) {
        errorMessage = error.getUserMessage();
      } else if (error.message) {
        errorMessage = error.message;
      }
      searchResultsList.innerHTML =
        `<div style="padding: 20px; text-align: center; color: var(--text-secondary);">${escapeHtml(errorMessage)}</div>`;
    }
    let errorMessage = "Unknown error";
    if (error instanceof APIError) {
      errorMessage = error.getUserMessage();
    } else if (error.message) {
      errorMessage = error.message;
    }
    showToast(`Search failed: ${errorMessage}`, "error");
  }
}

// Navigate through search results with arrow keys
function navigateSearchResults(direction) {
  if (currentSearchResults.length === 0) return;

  selectedSearchIndex += direction;

  if (selectedSearchIndex < 0) {
    selectedSearchIndex = currentSearchResults.length - 1;
  } else if (selectedSearchIndex >= currentSearchResults.length) {
    selectedSearchIndex = 0;
  }

  updateSearchSelection();
}

// Update visual selection in search results
function updateSearchSelection() {
  if (!searchResultsList) return;

  const items = searchResultsList.querySelectorAll(".search-result-item");
  items.forEach((item, index) => {
    if (index === selectedSearchIndex) {
      item.classList.add("selected");
      item.scrollIntoView({ block: "nearest", behavior: "smooth" });
    } else {
      item.classList.remove("selected");
    }
  });
}

// Navigate to selected search result
function navigateToSearchResult(file) {
  if (!file) return;

  closeSearchModal();

  // Navigate to the file's collection
  const fileType = file.type || file.original_name;
  const collection = getCollectionFromFileType(fileType);

  if (collection) {
    currentCollectionType = collection;
    showPage(collection === "images" ? "images" : "images"); // Use images page for now
    loadCollectionFiles(collection);
    showToast(`Opening ${file.original_name || "file"}`);
  } else {
    showToast(`File found: ${file.original_name || "Unnamed file"}`);
  }

  // Clear search input
  if (searchInput) searchInput.value = "";
}

// Close search modal
function closeSearchModal() {
  if (searchModal) {
    searchModal.style.display = "none";
  }
  selectedSearchIndex = -1;
  currentSearchResults = [];
}

// Get collection from file type
function getCollectionFromFileType(fileName) {
  const ext = fileName.toLowerCase().split(".").pop();

  const collections = {
    images: ["jpg", "jpeg", "png", "gif", "bmp", "svg", "webp", "ico"],
    videos: ["mp4", "avi", "mov", "wmv", "flv", "webm", "mkv"],
    audio: ["mp3", "wav", "ogg", "flac", "m4a", "aac"],
    documents: ["pdf", "doc", "docx", "txt", "rtf", "odt"],
    spreadsheets: ["xls", "xlsx", "csv", "ods"],
    presentations: ["ppt", "pptx", "odp"],
    code: ["js", "py", "java", "cpp", "c", "h", "css", "html", "json", "xml"],
  };

  for (const [collection, extensions] of Object.entries(collections)) {
    if (extensions.includes(ext)) {
      return collection;
    }
  }

  return "documents"; // Default fallback
}

// Get file icon based on file type
function getFileIcon(fileName) {
  const ext = fileName.toLowerCase().split(".").pop();

  if (["jpg", "jpeg", "png", "gif", "bmp", "svg", "webp"].includes(ext)) {
    return `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
    </svg>`;
  } else if (["mp4", "avi", "mov", "wmv", "flv", "webm"].includes(ext)) {
    return `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="m15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z" />
    </svg>`;
  } else if (["mp3", "wav", "ogg", "flac"].includes(ext)) {
    return `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="m9 9 10.5-3m0 6.553v3.75a2.25 2.25 0 0 1-1.632 2.163l-1.32.377a1.803 1.803 0 1 1-.99-3.467l2.31-.66a2.25 2.25 0 0 0 1.632-2.163Zm0 0V2.25L9 5.25v10.303m0 0v3.75a2.25 2.25 0 0 1-1.632 2.163l-1.32.377a1.803 1.803 0 0 1-.99-3.467l2.31-.66A2.25 2.25 0 0 0 9 15.553Z" />
    </svg>`;
  } else if (["pdf", "doc", "docx", "txt"].includes(ext)) {
    return `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
    </svg>`;
  } else {
    return `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
    </svg>`;
  }
}

// Format file size
function formatFileSize(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

// Collection card navigation
function initCollectionCards() {
  const collectionCardButtons = document.querySelectorAll(".collection-card");
  collectionCardButtons.forEach((card) => {
    card.addEventListener("click", () => {
      const collection = card.dataset.collection;
      currentCollectionType = collection;

      // Navigate to collection page
      const collectionPage = document.getElementById(`page-${collection}`);
      if (collectionPage) {
        showPage(collection);
        loadCollectionFiles(collection);
      } else {
        // If page doesn't exist, create it dynamically or use images page
        showPage("images");
        loadCollectionFiles(collection);
      }
    });
  });
}

// Load files for a collection from API
async function loadCollectionFiles(collectionType) {
  const gallery = document.getElementById("files-gallery");
  const loadingState = document.getElementById("gallery-loading");
  const emptyState = document.getElementById("gallery-empty");

  if (!gallery) return;

  try {
    // Show loading state
    gallery.innerHTML = "";
    if (loadingState) loadingState.style.display = "block";
    if (emptyState) emptyState.style.display = "none";

    // Map collection types to API types
    const apiTypeMap = {
      images: "images",
      videos: "videos",
      audio: "audio",
      documents: "documents",
      spreadsheets: "documents",
      presentations: "documents",
      archives: "archives",
      other: "other",
    };

    const apiType = apiTypeMap[collectionType] || collectionType;

    // Fetch files from API
    const response = await getFiles(apiType);
    const files = response.files || response || [];

    // Hide loading state
    if (loadingState) loadingState.style.display = "none";

    if (files.length === 0) {
      if (emptyState) emptyState.style.display = "block";
      return;
    }

    if (emptyState) emptyState.style.display = "none";

    // Render files
    files.forEach((file) => {
      const fileElement = createFileElement(file, collectionType);
      gallery.appendChild(fileElement);
    });

    // Re-initialize gallery menus for new elements
    initGalleryMenus();
  } catch (error) {
    console.error("Error loading files:", error);
    if (loadingState) loadingState.style.display = "none";
    if (emptyState) {
      let errorMessage = "Error loading files. Please try again.";
      if (error instanceof APIError) {
        errorMessage = error.getUserMessage();
      } else if (error.message) {
        errorMessage = error.message;
      }
      emptyState.innerHTML = `<p>${escapeHtml(errorMessage)}</p>`;
      emptyState.style.display = "block";
    }
    let errorMessage = "Failed to load files";
    if (error instanceof APIError) {
      errorMessage = error.getUserMessage();
    } else if (error.message) {
      errorMessage = error.message;
    }
    showToast(errorMessage, "error", 0, [{
      id: "retry",
      label: "Retry",
      handler: () => loadCollectionFiles(collectionType),
    }]);
  }
}

// Create a file element for the gallery
function createFileElement(file, collectionType) {
  const div = document.createElement("div");
  div.className = "gallery-item";
  div.dataset.fileId = file.id || file.fileId || `file-${Date.now()}`;
  div.dataset.fileName = file.name || file.fileName || "Untitled";
  div.dataset.filePath = file.path || file.filePath || "";
  div.dataset.fileUrl = file.url || file.downloadUrl || file.path || "";
  div.dataset.fileDate =
    file.date || file.uploadedAt || new Date().toISOString();
  div.dataset.fileSize = file.size || file.fileSize || "Unknown";
  div.dataset.fileType = file.type || file.fileType || "Unknown";
  div.dataset.fileDimensions = file.dimensions || file.fileDimensions || "";

  const isImage =
    collectionType === "images" || file.type?.startsWith("image/");
  const imageUrl = file.url || file.path || file.thumbnail || "";

  div.innerHTML = `
    <div class="gallery-item-header">
      <div class="gallery-image-container">
        ${
          isImage
            ? `
          <img
            src="${imageUrl}"
            alt="${file.name || "File"}"
            loading="lazy"
            class="gallery-image"
            onerror="this.src='data:image/svg+xml,%3Csvg xmlns=\\'http://www.w3.org/2000/svg\\' viewBox=\\'0 0 100 100\\'%3E%3Crect fill=\\'%23ddd\\' width=\\'100\\' height=\\'100\\'/%3E%3Ctext x=\\'50\\' y=\\'50\\' text-anchor=\\'middle\\' dy=\\'.3em\\' font-size=\\'14\\' fill=\\'%23999\\'%3E${
              file.type || "File"
            }%3C/text%3E%3C/svg%3E'"
          />
        `
            : `
          <div style="display: flex; align-items: center; justify-content: center; height: 100%; background: var(--surface-muted);">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" style="width: 48px; height: 48px; color: var(--text-secondary);">
              <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
            </svg>
          </div>
        `
        }
      </div>
      <button type="button" class="gallery-menu-button" aria-label="File options">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.75a.75.75 0 110-1.5.75.75 0 010 1.5zM12 12.75a.75.75 0 110-1.5.75.75 0 010 1.5zM12 18.75a.75.75 0 110-1.5.75.75 0 010 1.5z" />
        </svg>
      </button>
      <div class="gallery-menu-dropdown" style="display: none;">
        <button type="button" class="menu-option" data-action="rename">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10" />
          </svg>
          Rename
        </button>
        <button type="button" class="menu-option menu-option-with-info" data-action="info">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" />
          </svg>
          Info
          <div class="file-info-tooltip">
            <div class="info-row">
              <span class="info-label">Date Uploaded:</span>
              <span class="info-value" data-info="date">N/A</span>
            </div>
            <div class="info-row">
              <span class="info-label">Path:</span>
              <span class="info-value" data-info="path">N/A</span>
            </div>
            <div class="info-row">
              <span class="info-label">Size:</span>
              <span class="info-value" data-info="size">N/A</span>
            </div>
            <div class="info-row">
              <span class="info-label">File Type:</span>
              <span class="info-value" data-info="type">N/A</span>
            </div>
            <div class="info-row">
              <span class="info-label">Dimensions:</span>
              <span class="info-value" data-info="dimensions">N/A</span>
            </div>
          </div>
        </button>
        <button type="button" class="menu-option" data-action="comments">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L6.832 19.82a4.5 4.5 0 01-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 011.13-1.897L16.863 4.487zm0 0L19.5 7.125" />
          </svg>
          Notes
        </button>
        <button type="button" class="menu-option" data-action="delete">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
          </svg>
          Delete
        </button>
        <button type="button" class="menu-option" data-action="download">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
          </svg>
          Download
        </button>
        <button type="button" class="menu-option" data-action="copy-path">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.6A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.6m5.332 0A2.251 2.251 0 0113.5 4.25h3a2.25 2.25 0 010 4.5h-3a2.25 2.25 0 00-2.166 1.6m5.332 0a2.251 2.251 0 01-.833 2.4m5.332 0A2.251 2.251 0 0118 15.75h-3a2.25 2.25 0 01-2.166-1.6M15.666 3.6a2.25 2.25 0 00-2.166 1.6M15.666 3.6v1.5m-5.332 0V3.6m0 0a2.25 2.25 0 00-2.166 1.6m5.332 0H9.75" />
          </svg>
          Copy Path
        </button>
      </div>
    </div>
    <div class="gallery-item-info">
      <h3 class="gallery-item-title">${escapeHtml(
        file.name || file.fileName || "Untitled"
      )}</h3>
      <p>${escapeHtml(file.description || file.comment || "")}</p>
      <span class="gallery-item-meta">${
        file.size || file.fileSize || "Unknown"
      } • ${file.type || file.fileType || "Unknown"}</span>
    </div>
  `;

  return div;
}

// Load collections from backend and render cards
async function loadCollections() {
  const collectionCards = document.getElementById("collectionCards");
  const loadingState = document.getElementById("collections-loading");
  const errorState = document.getElementById("collections-error");

  if (!collectionCards) return;

  try {
    // Show loading state
    if (loadingState) loadingState.style.display = "block";
    if (errorState) errorState.style.display = "none";

    // Clear existing cards (except loading/error states)
    const existingCards = collectionCards.querySelectorAll(".collection-card");
    existingCards.forEach((card) => card.remove());

    // Fetch collections from API
    const response = await getCollections();
    const collections = response.collections || response || [];

    if (collections.length === 0) {
      if (loadingState) loadingState.style.display = "none";
      collectionCards.innerHTML =
        '<p style="padding: 20px; text-align: center; color: var(--text-secondary);">No collections available</p>';
      return;
    }

    // Fetch stats for each collection in parallel
    const statsPromises = collections.map((collection) =>
      getCollectionStats(collection.type).catch((err) => {
        console.warn(`Failed to fetch stats for ${collection.type}:`, err);
        return {
          type: collection.type,
          file_count: 0,
          storage_used: 0,
          storage_used_formatted: "0 B",
        };
      })
    );

    const statsResults = await Promise.all(statsPromises);
    const statsMap = new Map();
    statsResults.forEach((stats) => {
      statsMap.set(stats.type, stats);
    });

    // Hide loading state
    if (loadingState) loadingState.style.display = "none";

    // Render collection cards
    collections.forEach((collection) => {
      const stats = statsMap.get(collection.type) || {
        file_count: 0,
        storage_used_formatted: "0 B",
      };
      const card = createCollectionCard(collection, stats);
      collectionCards.appendChild(card);
    });

    // Re-initialize collection card click handlers
    initCollectionCards();
  } catch (error) {
    console.error("Error loading collections:", error);
    if (loadingState) loadingState.style.display = "none";
    if (errorState) {
      let errorMessage = "Unknown error";
      if (error instanceof APIError) {
        errorMessage = error.getUserMessage();
      } else if (error.message) {
        errorMessage = error.message;
      }
      errorState.style.display = "block";
      errorState.innerHTML = `<p>Failed to load collections: ${escapeHtml(errorMessage)}</p>`;
    }
    let errorMessage = "Failed to load collections";
    if (error instanceof APIError) {
      errorMessage = error.getUserMessage();
    } else if (error.message) {
      errorMessage = error.message;
    }
    showToast(errorMessage, "error", 0, [{
      id: "retry",
      label: "Retry",
      handler: () => loadCollections(),
    }]);
  }
}

// Create a collection card element
function createCollectionCard(collection, stats) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "collection-card";
  button.dataset.collection = collection.type;

  // Map collection types to image URLs
  const imageMap = {
    images:
      "https://images.unsplash.com/photo-1516035069371-29a1b244cc32?auto=format&fit=crop&w=600&q=80",
    videos:
      "https://images.unsplash.com/photo-1533750516457-a7f992034fec?auto=format&fit=crop&w=600&q=80",
    audio:
      "https://images.unsplash.com/photo-1493225457124-a3eb161ffa5f?auto=format&fit=crop&w=600&q=80",
    documents:
      "https://images.unsplash.com/photo-1455390582262-044cdead277a?auto=format&fit=crop&w=600&q=80",
    spreadsheets:
      "https://images.unsplash.com/photo-1551288049-bebda4e38f71?auto=format&fit=crop&w=600&q=80",
    presentations:
      "https://images.unsplash.com/photo-1554224155-6726b3ff858f?auto=format&fit=crop&w=600&q=80",
    archives:
      "https://images.unsplash.com/photo-1586281380349-632531db7ed4?auto=format&fit=crop&w=600&q=80",
    other:
      "https://images.unsplash.com/photo-1558494949-ef010cbdcc31?auto=format&fit=crop&w=600&q=80",
  };

  const imageUrl = imageMap[collection.type] || imageMap["other"];
  const fileCount = stats.file_count || 0;
  const storageUsed =
    stats.storage_used_formatted || stats.storage_used || "0 B";

  button.innerHTML = `
    <img
      src="${imageUrl}"
      alt="${collection.name || collection.type}"
      loading="lazy"
    />
    <div class="collection-meta">
      <h3>${escapeHtml(collection.name || collection.type)}</h3>
      <p>${escapeHtml(collection.description || "")}</p>
      <div class="collection-stats">
        <span class="stat-item">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" style="width: 16px; height: 16px; vertical-align: middle;">
            <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z" />
          </svg>
          ${fileCount.toLocaleString()} file${fileCount !== 1 ? "s" : ""}
        </span>
        <span class="stat-item">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" style="width: 16px; height: 16px; vertical-align: middle;">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
          </svg>
          ${escapeHtml(storageUsed)}
        </span>
      </div>
    </div>
  `;

  return button;
}

// Collection cards initialization is now in initAll()

// Gallery menu functionality
function initGalleryMenus() {
  const menuButtons = document.querySelectorAll(".gallery-menu-button");
  const menuOptions = document.querySelectorAll(".menu-option");

  // Close all dropdowns when clicking outside
  document.addEventListener("click", (e) => {
    if (
      !e.target.closest(".gallery-menu-button") &&
      !e.target.closest(".gallery-menu-dropdown")
    ) {
      document
        .querySelectorAll(".gallery-menu-dropdown")
        .forEach((dropdown) => {
          dropdown.style.display = "none";
          dropdown.classList.remove("is-visible");
        });
    }
  });

  // Toggle dropdown on menu button click
  menuButtons.forEach((button) => {
    button.addEventListener("click", (e) => {
      e.stopPropagation();
      const dropdown = button.nextElementSibling;
      const isVisible =
        dropdown.style.display === "flex" ||
        dropdown.classList.contains("is-visible");

      // Close all other dropdowns
      document.querySelectorAll(".gallery-menu-dropdown").forEach((d) => {
        d.style.display = "none";
        d.classList.remove("is-visible");
      });

      // Toggle current dropdown
      if (!isVisible) {
        dropdown.style.display = "flex";
        dropdown.classList.add("is-visible");
      }
    });
  });

  // Handle menu option clicks
  menuOptions.forEach((option) => {
    option.addEventListener("click", async (e) => {
      e.stopPropagation();
      const action = option.dataset.action;
      const galleryItem = option.closest(".gallery-item");
      const fileId = galleryItem.dataset.fileId;
      const fileName = galleryItem.dataset.fileName;
      const filePath = galleryItem.dataset.filePath;
      const fileUrl = galleryItem.dataset.fileUrl;
      const titleElement = galleryItem.querySelector(".gallery-item-title");

      // Close dropdown
      const dropdown = option.closest(".gallery-menu-dropdown");
      dropdown.style.display = "none";
      dropdown.classList.remove("is-visible");

      if (action === "download") {
        e.preventDefault();
        try {
          const downloadToast = showToast(`Downloading "${fileName}"...`, "info", 0);
          await downloadFile(fileId, fileName, fileUrl, filePath);
          dismissToast(downloadToast);
          showToast(`Download started for "${fileName}"`, "success");
        } catch (error) {
          console.error("Download error:", error);
          let errorMessage = "Unknown error";
          if (error instanceof APIError) {
            errorMessage = error.getUserMessage();
          } else if (error.message) {
            errorMessage = error.message;
          }
          showToast(`Failed to download: ${errorMessage}`, "error", 0, [{
            id: "retry",
            label: "Retry",
            handler: () => {
              const actionEvent = new Event("click");
              option.dispatchEvent(actionEvent);
            },
          }]);
        }
      } else if (action === "rename") {
        e.preventDefault();
        const newName = prompt("Enter new name:", fileName);
        if (newName && newName.trim() && newName !== fileName) {
          try {
            await renameFile(fileId, newName.trim());
            titleElement.textContent = newName.trim();
            galleryItem.dataset.fileName = newName.trim();
            showToast(`Renamed to "${newName.trim()}"`, "success");
          } catch (error) {
            console.error("Rename error:", error);
            let errorMessage = "Unknown error";
            if (error instanceof APIError) {
              errorMessage = error.getUserMessage();
            } else if (error.message) {
              errorMessage = error.message;
            }
            showToast(`Failed to rename: ${errorMessage}`, "error");
          }
        }
      } else if (action === "delete") {
        e.preventDefault();
        if (confirm(`Are you sure you want to delete "${fileName}"?`)) {
          try {
            await deleteFile(fileId);
            galleryItem.style.opacity = "0";
            galleryItem.style.transform = "scale(0.95)";
            setTimeout(() => {
              galleryItem.remove();
              showToast(`Deleted "${fileName}"`, "success");
            }, 200);
          } catch (error) {
            console.error("Delete error:", error);
            let errorMessage = "Unknown error";
            if (error instanceof APIError) {
              errorMessage = error.getUserMessage();
            } else if (error.message) {
              errorMessage = error.message;
            }
            showToast(`Failed to delete: ${errorMessage}`, "error");
          }
        }
      } else if (action === "copy-path") {
        e.preventDefault();
        navigator.clipboard
          .writeText(filePath)
          .then(() => {
            showToast("Path copied to clipboard");
          })
          .catch(() => {
            // Fallback for older browsers
            const textArea = document.createElement("textarea");
            textArea.value = filePath;
            document.body.appendChild(textArea);
            textArea.select();
            document.execCommand("copy");
            document.body.removeChild(textArea);
            showToast("Path copied to clipboard");
          });
      } else if (action === "comments") {
        e.preventDefault();
        try {
          await openCommentsModal(galleryItem);
        } catch (error) {
          console.error("Error opening comments modal:", error);
          let errorMessage = "Unknown error";
          if (error instanceof APIError) {
            errorMessage = error.getUserMessage();
          } else if (error.message) {
            errorMessage = error.message;
          }
          showToast(`Failed to open notes: ${errorMessage}`, "error");
        }
      }
    });
  });

  // Populate tooltip with file data on hover
  const infoOptions = document.querySelectorAll(".menu-option-with-info");
  infoOptions.forEach((option) => {
    option.addEventListener("mouseenter", () => {
      const galleryItem = option.closest(".gallery-item");
      if (!galleryItem) return;

      const tooltip = option.querySelector(".file-info-tooltip");
      if (!tooltip) return;

      // Get data from gallery item
      const fileDate = galleryItem.dataset.fileDate || "";
      const filePath = galleryItem.dataset.filePath || "";
      const fileSize = galleryItem.dataset.fileSize || "";
      const fileType = galleryItem.dataset.fileType || "";
      const fileDimensions = galleryItem.dataset.fileDimensions || "";

      // Format date
      let formattedDate = fileDate;
      if (fileDate) {
        try {
          const date = new Date(fileDate);
          if (!isNaN(date.getTime())) {
            formattedDate = date.toLocaleDateString("en-US", {
              year: "numeric",
              month: "long",
              day: "numeric",
            });
          }
        } catch (e) {
          // Keep original date if parsing fails
        }
      }

      // Update tooltip values
      const dateValue = tooltip.querySelector('[data-info="date"]');
      const pathValue = tooltip.querySelector('[data-info="path"]');
      const sizeValue = tooltip.querySelector('[data-info="size"]');
      const typeValue = tooltip.querySelector('[data-info="type"]');
      const dimensionsValue = tooltip.querySelector('[data-info="dimensions"]');

      if (dateValue) dateValue.textContent = formattedDate || "N/A";
      if (pathValue) {
        // Truncate long paths
        const maxLength = 30;
        pathValue.textContent =
          filePath.length > maxLength
            ? filePath.substring(0, maxLength) + "..."
            : filePath || "N/A";
      }
      if (sizeValue) sizeValue.textContent = fileSize || "N/A";
      if (typeValue) typeValue.textContent = fileType || "N/A";
      if (dimensionsValue)
        dimensionsValue.textContent = fileDimensions || "N/A";
    });
  });
}

// Initialize gallery menus when DOM is ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", initGalleryMenus);
} else {
  initGalleryMenus();
}

// Initialize ghost button
function initGhostButton() {
  const ghostButton = document.querySelector(".ghost-button");
  if (ghostButton && !ghostButton.hasAttribute("data-listener-attached")) {
    ghostButton.setAttribute("data-listener-attached", "true");
    ghostButton.addEventListener("click", () => {
      showToast("Viewing all collections");
      // Could show all collections or filter view
    });
  }
}

// Download file function
async function downloadFile(fileId, fileName, fileUrl, filePath) {
  try {
    // Try to get file from API first to get download URL
    let downloadUrl = fileUrl;

    // If no direct URL, try to construct download URL from backend
    if (!downloadUrl || downloadUrl === "") {
      // Try to fetch file info from API to get download URL
      try {
        const fileInfo = await getFile(fileId);
        downloadUrl =
          fileInfo.url || fileInfo.downloadUrl || fileInfo.path || downloadUrl;
      } catch (error) {
        console.warn(
          "Could not fetch file info, trying direct download:",
          error
        );
        // Construct download URL from backend
        downloadUrl = `${API_CONFIG.baseURL}/files/${fileId}/download`;
      }
    }

    // If still no URL, use the file path or construct from fileId
    if (!downloadUrl || downloadUrl === "") {
      downloadUrl =
        filePath || `${API_CONFIG.baseURL}/files/${fileId}/download`;
    }

    // Create a temporary anchor element to trigger download
    const link = document.createElement("a");
    link.href = downloadUrl;
    link.download = fileName || "download";
    link.style.display = "none";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);

    // If direct link doesn't work, try fetching as blob
    setTimeout(async () => {
      try {
        const response = await fetch(downloadUrl, {
          method: "GET",
          headers: getHeaders(),
        });

        if (response.ok) {
          const blob = await response.blob();
          const blobUrl = window.URL.createObjectURL(blob);
          const link = document.createElement("a");
          link.href = blobUrl;
          link.download = fileName || "download";
          document.body.appendChild(link);
          link.click();
          document.body.removeChild(link);
          window.URL.revokeObjectURL(blobUrl);
        }
      } catch (error) {
        console.error("Blob download failed:", error);
      }
    }, 100);
  } catch (error) {
    console.error("Download error:", error);
    throw error;
  }
}

// Helper function to get headers (imported from api.js context)
function getHeaders() {
  const headers = {};
  const token =
    localStorage.getItem("auth_token") || sessionStorage.getItem("auth_token");
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }
  return headers;
}

// Ensure all buttons have proper cursor and are clickable
function ensureButtonsClickable() {
  document.querySelectorAll("button").forEach((btn) => {
    if (!btn.style.cursor) {
      btn.style.cursor = "pointer";
    }
    // Ensure buttons are not disabled by CSS
    btn.style.pointerEvents = "auto";
  });
}

// File validation constants
const MAX_FILE_SIZE = 500 * 1024 * 1024; // 500MB
const SUPPORTED_IMAGE_TYPES = ['image/jpeg', 'image/jpg', 'image/png', 'image/gif', 'image/webp', 'image/svg+xml', 'image/bmp'];
const SUPPORTED_VIDEO_TYPES = ['video/mp4', 'video/webm', 'video/ogg', 'video/quicktime', 'video/x-msvideo'];
const SUPPORTED_AUDIO_TYPES = ['audio/mpeg', 'audio/wav', 'audio/ogg', 'audio/flac', 'audio/aac'];
const SUPPORTED_DOCUMENT_TYPES = ['application/pdf', 'application/msword', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'text/plain', 'text/markdown'];
const ALL_SUPPORTED_TYPES = [...SUPPORTED_IMAGE_TYPES, ...SUPPORTED_VIDEO_TYPES, ...SUPPORTED_AUDIO_TYPES, ...SUPPORTED_DOCUMENT_TYPES];

/**
 * Validate file before upload
 */
function validateFile(file) {
  const errors = [];

  // Check file size
  if (file.size > MAX_FILE_SIZE) {
    errors.push({
      type: 'file-too-large',
      message: `File "${file.name}" exceeds the maximum size limit of 500MB.`,
    });
  }

  // Check file type (optional - allow all types but warn)
  if (file.type && ALL_SUPPORTED_TYPES.length > 0 && !ALL_SUPPORTED_TYPES.includes(file.type)) {
    // Don't block, just note it
    console.warn(`File type ${file.type} may not be fully supported`);
  }

  return errors;
}

/**
 * Upload files to backend with progress and error handling
 */
async function uploadFiles(files) {
  if (!files || files.length === 0) {
    showToast("No files selected", "warning");
    return;
  }

  // Validate all files first
  const validationErrors = [];
  const validFiles = [];

  for (const file of files) {
    const errors = validateFile(file);
    if (errors.length > 0) {
      validationErrors.push(...errors);
    } else {
      validFiles.push(file);
    }
  }

  // Show validation errors
  if (validationErrors.length > 0) {
    validationErrors.forEach((error) => {
      showToast(error.message, "error");
    });
    
    if (validFiles.length === 0) {
      return; // No valid files to upload
    }
  }

  // Show upload started notification
  const uploadToast = showToast(
    `Uploading ${validFiles.length} file${validFiles.length > 1 ? "s" : ""}...`,
    "info",
    0 // Don't auto-dismiss
  );

  const uploadResults = {
    success: [],
    failed: [],
  };

  try {
    // Determine if files are media or mixed
    const mediaTypes = ["image/", "video/", "audio/"];
    const allMedia = validFiles.every((file) =>
      mediaTypes.some((type) => file.type && file.type.startsWith(type))
    );

    let result;
    if (allMedia && validFiles.length > 0) {
      // Use media endpoint for media files
      result = await ingestMedia(validFiles);
    } else {
      // Use unified endpoint for mixed files
      result = await ingestFiles(validFiles);
    }

    // Dismiss upload toast
    dismissToast(uploadToast);

    // Show success
    showToast(
      `Successfully uploaded ${validFiles.length} file${validFiles.length > 1 ? "s" : ""}`,
      "success"
    );

    uploadResults.success = validFiles;

    // Reload current collection if viewing one
    if (currentCollectionType) {
      loadCollectionFiles(currentCollectionType);
    }
  } catch (error) {
    console.error("Upload error:", error);
    
    // Dismiss upload toast
    dismissToast(uploadToast);

    // Import APIError if available
    let errorMessage = "Unknown error";
    let errorType = "error";
    let retryAction = null;

    if (error && typeof error === 'object') {
      // Check if it's an APIError
      if (error.getUserMessage && typeof error.getUserMessage === 'function') {
        errorMessage = error.getUserMessage();
        errorType = error.getErrorType ? error.getErrorType() : 'error';
      } else if (error.message) {
        errorMessage = error.message;
      }

      // Determine error type for better messaging
      if (errorMessage.includes("Cannot connect") || errorMessage.includes("Failed to fetch") || errorMessage.includes("Network")) {
        errorMessage = "Cannot connect to backend. Please ensure the server is running.";
        errorType = "network";
        retryAction = {
          id: "retry",
          label: "Retry",
          handler: () => uploadFiles(validFiles),
        };
      } else if (errorMessage.includes("413") || errorMessage.includes("too large")) {
        errorMessage = "File exceeds the maximum size limit (500MB).";
        errorType = "file-too-large";
      } else if (errorMessage.includes("415") || errorMessage.includes("unsupported")) {
        errorMessage = "File format not supported.";
        errorType = "unsupported-format";
      }
    }

    // Show error toast with retry option
    showToast(
      `Upload failed: ${errorMessage}`,
      "error",
      0, // Manual dismiss
      retryAction ? [retryAction] : []
    );

    uploadResults.failed = validFiles;
  }

  return uploadResults;
}

// ==================== Toast Notification System ====================

const TOAST_CONTAINER_ID = 'toast-container';
const MAX_TOASTS = 5;
const TOAST_DURATIONS = {
  success: 3000,
  error: 0, // Manual dismiss for errors
  info: 5000,
  warning: 4000,
};

let toastContainer = null;
let activeToasts = [];

/**
 * Initialize toast container
 */
function initToastContainer() {
  if (!toastContainer) {
    // Check if container already exists
    toastContainer = document.getElementById(TOAST_CONTAINER_ID);
    
    if (!toastContainer) {
      // Create toast container
      toastContainer = document.createElement('div');
      toastContainer.id = TOAST_CONTAINER_ID;
      toastContainer.className = 'toast-container';
      toastContainer.setAttribute('aria-live', 'polite');
      toastContainer.setAttribute('aria-atomic', 'false');
      document.body.appendChild(toastContainer);
    }
  }
  return toastContainer;
}

/**
 * Get icon for toast type
 */
function getToastIcon(type) {
  const icons = {
    success: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>`,
    error: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
    </svg>`,
    info: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" />
    </svg>`,
    warning: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.008v.008H12v-.008z" />
    </svg>`,
  };
  return icons[type] || icons.info;
}

/**
 * Show toast notification
 * @param {string} message - Toast message
 * @param {string} type - Toast type: 'success', 'error', 'info', 'warning'
 * @param {number} duration - Auto-dismiss duration in ms (0 = manual dismiss)
 * @param {Array} actions - Array of action buttons: [{ id, label, handler }]
 * @returns {HTMLElement} Toast element
 */
function showToast(message, type = 'info', duration = null, actions = []) {
  const container = initToastContainer();
  
  // Remove oldest toast if at max capacity
  if (activeToasts.length >= MAX_TOASTS) {
    const oldestToast = activeToasts.shift();
    dismissToast(oldestToast);
  }
  
  // Use default duration if not specified
  if (duration === null) {
    duration = TOAST_DURATIONS[type] || TOAST_DURATIONS.info;
  }
  
  // Create toast element
  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.setAttribute('role', type === 'error' ? 'alert' : 'status');
  toast.setAttribute('aria-live', type === 'error' ? 'assertive' : 'polite');
  
  const icon = getToastIcon(type);
  const actionsHTML = actions
    .map(
      (a) =>
        `<button class="toast-action" data-action="${escapeHtml(a.id)}" aria-label="${escapeHtml(a.label)}">${escapeHtml(a.label)}</button>`
    )
    .join('');
  
  toast.innerHTML = `
    <div class="toast-icon">${icon}</div>
    <div class="toast-content">
      <div class="toast-message">${escapeHtml(message)}</div>
      ${actionsHTML ? `<div class="toast-actions">${actionsHTML}</div>` : ''}
    </div>
    <button class="toast-close" aria-label="Close" type="button">&times;</button>
  `;
  
  // Add to container and active list
  container.appendChild(toast);
  activeToasts.push(toast);
  
  // Trigger animation
  requestAnimationFrame(() => {
    toast.classList.add('is-visible');
  });
  
  // Handle close button
  const closeButton = toast.querySelector('.toast-close');
  closeButton.addEventListener('click', () => {
    dismissToast(toast);
  });
  
  // Handle action buttons
  actions.forEach((action) => {
    const actionButton = toast.querySelector(`[data-action="${action.id}"]`);
    if (actionButton && action.handler) {
      actionButton.addEventListener('click', () => {
        action.handler();
        if (action.dismissOnClick !== false) {
          dismissToast(toast);
        }
      });
    }
  });
  
  // Auto-dismiss for non-error toasts
  if (type !== 'error' && duration > 0) {
    const timeoutId = setTimeout(() => {
      dismissToast(toast);
    }, duration);
    
    // Store timeout ID for potential cancellation
    toast.dataset.timeoutId = timeoutId;
  }
  
  return toast;
}

/**
 * Dismiss a toast
 */
function dismissToast(toast) {
  if (!toast || !toast.parentNode) return;
  
  // Clear timeout if exists
  if (toast.dataset.timeoutId) {
    clearTimeout(parseInt(toast.dataset.timeoutId, 10));
  }
  
  // Remove from active list
  const index = activeToasts.indexOf(toast);
  if (index > -1) {
    activeToasts.splice(index, 1);
  }
  
  // Animate out
  toast.classList.remove('is-visible');
  toast.classList.add('is-dismissing');
  
  setTimeout(() => {
    if (toast.parentNode) {
      toast.parentNode.removeChild(toast);
    }
  }, 300);
}

/**
 * Dismiss all toasts
 */
function dismissAllToasts() {
  activeToasts.forEach((toast) => dismissToast(toast));
  activeToasts = [];
}

// ==================== Loading Overlay System ====================

let loadingOverlay = null;
let loadingOverlayCount = 0;

/**
 * Show global loading overlay
 * @param {string} message - Optional loading message
 */
function showLoadingOverlay(message = 'Loading...') {
  if (!loadingOverlay) {
    loadingOverlay = document.createElement('div');
    loadingOverlay.id = 'loading-overlay';
    loadingOverlay.className = 'loading-overlay';
    loadingOverlay.setAttribute('role', 'status');
    loadingOverlay.setAttribute('aria-live', 'polite');
    loadingOverlay.setAttribute('aria-label', message);
    document.body.appendChild(loadingOverlay);
  }

  loadingOverlay.innerHTML = `
    <div class="loading-spinner">
      <div class="spinner-ring"></div>
      <div class="spinner-text">${escapeHtml(message)}</div>
    </div>
  `;

  loadingOverlayCount++;
  loadingOverlay.style.display = 'flex';
}

/**
 * Hide global loading overlay
 */
function hideLoadingOverlay() {
  if (loadingOverlayCount > 0) {
    loadingOverlayCount--;
  }

  if (loadingOverlayCount === 0 && loadingOverlay) {
    loadingOverlay.style.display = 'none';
  }
}

/**
 * Wrap async function with loading overlay
 */
async function withLoadingOverlay(asyncFn, message = 'Loading...') {
  showLoadingOverlay(message);
  try {
    const result = await asyncFn();
    return result;
  } finally {
    hideLoadingOverlay();
  }
}

/**
 * Legacy showToast function for backward compatibility
 * @deprecated Use showToast(message, type, duration, actions) instead
 */
let toastTimeoutId;
function showToastLegacy(message) {
  showToast(message, 'info');
}

// Initialize all features when DOM is ready
function initAll() {
  try {
    toast = document.getElementById("toast");

    // Initialize theme toggle FIRST so modeToggle is available
    initThemeToggle();

    // Then initialize theme (which uses modeToggle)
    initTheme();

    // Initialize all other features
    initHomePageFeatures();
    initSidebarNavigation();
    initGlobalSearch();
    initCollectionCards();
    initGalleryMenus();
    initLayoutToggle();
    initCommentsModal();
    initGhostButton();
    initDataTabs();
    ensureButtonsClickable();

    // Load collections if on files page
    if (
      document.getElementById("page-files") &&
      document.getElementById("page-files").style.display !== "none"
    ) {
      loadCollections();
    }

    console.log("All features initialized successfully");
  } catch (error) {
    console.error("Error initializing features:", error);
    if (toast) {
      toast.textContent = "Error initializing page. Please refresh.";
      toast.hidden = false;
    }
  }
}

// Initialize layout toggle
function initLayoutToggle() {
  const layoutButtons = document.querySelectorAll(".layout-button");
  const collectionCards = document.getElementById("collectionCards");

  layoutButtons.forEach((button) => {
    button.addEventListener("click", () => {
      const layout = button.dataset.layout;
      layoutButtons.forEach((btn) => btn.classList.remove("is-active"));
      button.classList.add("is-active");

      if (collectionCards) {
        if (layout === "list") {
          collectionCards.classList.add("list-layout");
        } else {
          collectionCards.classList.remove("list-layout");
        }
        showToast(`Switched to ${layout} layout`);
      }
    });
  });
}

// Initialize data tabs (SQL/NoSQL)
let dataTabsInitialized = false;
function initDataTabs() {
  const dataTabs = document.querySelectorAll(".data-tab");
  const sqlSection = document.getElementById("data-sql");
  const nosqlSection = document.getElementById("data-nosql");

  if (!dataTabs.length || !sqlSection || !nosqlSection) {
    // Elements not found, try again after a short delay (max 10 retries)
    if (!dataTabsInitialized) {
      if (typeof initDataTabs.retryCount === "undefined") {
        initDataTabs.retryCount = 0;
      }
      if (initDataTabs.retryCount < 10) {
        initDataTabs.retryCount++;
        setTimeout(initDataTabs, 100);
      } else {
        console.warn("Data tabs elements not found after 10 retries");
      }
    }
    return;
  }

  // Only initialize once
  if (dataTabsInitialized) return;
  dataTabsInitialized = true;

  dataTabs.forEach((tab) => {
    // Remove any existing listeners by cloning
    const newTab = tab.cloneNode(true);
    tab.parentNode.replaceChild(newTab, tab);
    
    newTab.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      const tabType = newTab.dataset.tab;

      // Remove active class from all tabs
      document
        .querySelectorAll(".data-tab")
        .forEach((t) => t.classList.remove("is-active"));
      // Add active class to clicked tab
      newTab.classList.add("is-active");

      // Show/hide sections
      if (tabType === "sql") {
        sqlSection.style.display = "flex";
        nosqlSection.style.display = "none";
      } else if (tabType === "nosql") {
        sqlSection.style.display = "none";
        nosqlSection.style.display = "flex";
        // Update diagram colors when switching to NoSQL tab
        setTimeout(updateNosqlDiagramColors, 100);
        // Initialize zoom and pan when switching to NoSQL tab
        setTimeout(initNosqlZoomPan, 100);
      }
    });
  });
}

// Comments functionality - variables declared before use
let commentsModal = null;
let commentsList = null;
let commentsEmpty = null;
let commentInput = null;
let commentsFileName = null;
let currentFileId = null;

// Initialize comments modal
let commentsModalInitialized = false;
function initCommentsModal() {
  if (commentsModalInitialized) return; // Prevent multiple initializations

  commentsModal = document.getElementById("comments-modal");
  commentsList = document.getElementById("comments-list");
  commentsEmpty = document.getElementById("comments-empty");
  commentInput = document.getElementById("comment-input");
  const commentSubmit = document.getElementById("comment-submit");
  const commentCancel = document.getElementById("comment-cancel");
  const commentsCloseButton = document.querySelector(".comments-close-button");
  commentsFileName = document.querySelector(".comments-file-name");

  if (commentSubmit && !commentSubmit.hasAttribute("data-listener-attached")) {
    commentSubmit.setAttribute("data-listener-attached", "true");
    commentSubmit.addEventListener("click", () => {
      if (currentFileId && commentInput && commentInput.value.trim()) {
        addComment(currentFileId, commentInput.value);
      }
    });
  }

  if (commentCancel && !commentCancel.hasAttribute("data-listener-attached")) {
    commentCancel.setAttribute("data-listener-attached", "true");
    commentCancel.addEventListener("click", () => {
      closeCommentsModal();
    });
  }

  if (
    commentsCloseButton &&
    !commentsCloseButton.hasAttribute("data-listener-attached")
  ) {
    commentsCloseButton.setAttribute("data-listener-attached", "true");
    commentsCloseButton.addEventListener("click", () => {
      closeCommentsModal();
    });
  }

  if (commentsModal) {
    const overlay = commentsModal.querySelector(".comments-modal-overlay");
    if (overlay && !overlay.hasAttribute("data-listener-attached")) {
      overlay.setAttribute("data-listener-attached", "true");
      overlay.addEventListener("click", () => {
        closeCommentsModal();
      });
    }

    // Only add escape key listener once
    if (!window.escapeKeyListenerAdded) {
      window.escapeKeyListenerAdded = true;
      document.addEventListener("keydown", (e) => {
        if (
          e.key === "Escape" &&
          commentsModal &&
          commentsModal.style.display === "flex"
        ) {
          closeCommentsModal();
        }
      });
    }

    const modalContent = commentsModal.querySelector(".comments-modal-content");
    if (modalContent && !modalContent.hasAttribute("data-listener-attached")) {
      modalContent.setAttribute("data-listener-attached", "true");
      modalContent.addEventListener("click", (e) => {
        e.stopPropagation();
      });
    }
  }

  commentsModalInitialized = true;
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", initAll);
} else {
  initAll();
}

document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible" && toast && !toast.hidden) {
    toast.classList.remove("is-visible");
    toast.hidden = true;
  }
});

// Comments functionality - variables moved above initCommentsModal()

// Get notes from API
async function getNotesFromAPI(fileId) {
  try {
    const response = await getNotes(fileId);
    return response.notes || response || [];
  } catch (error) {
    console.error("Error fetching notes:", error);
    return [];
  }
}

// Format date for display
function formatCommentDate(dateString) {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now - date;
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "Just now";
  if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? "s" : ""} ago`;
  if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? "s" : ""} ago`;
  if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? "s" : ""} ago`;

  return date.toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

// Get user initials for avatar
function getUserInitials() {
  // You can get this from user profile or use a default
  return "AZ"; // Default to profile initials
}

// Render comments
async function renderComments(fileId) {
  if (!commentsList || !commentsEmpty) return;

  commentsList.innerHTML = "";
  commentsEmpty.style.display = "none";
  commentsList.style.display = "flex";

  // Show loading
  const loadingDiv = document.createElement("div");
  loadingDiv.textContent = "Loading notes...";
  loadingDiv.style.padding = "20px";
  loadingDiv.style.textAlign = "center";
  loadingDiv.style.color = "var(--text-secondary)";
  commentsList.appendChild(loadingDiv);

  try {
    const notes = await getNotesFromAPI(fileId);

    commentsList.innerHTML = "";

    if (notes.length === 0) {
      commentsEmpty.style.display = "flex";
      commentsList.style.display = "none";
    } else {
      commentsEmpty.style.display = "none";
      commentsList.style.display = "flex";

      // Sort notes by date (newest first)
      const sortedNotes = [...notes].sort((a, b) => {
        const dateA = new Date(a.date || a.createdAt || a.timestamp);
        const dateB = new Date(b.date || b.createdAt || b.timestamp);
        return dateB - dateA;
      });

      sortedNotes.forEach((note) => {
        const commentItem = document.createElement("div");
        commentItem.className = "comment-item";
        commentItem.dataset.commentId = note.id || note.noteId;

        const initials = getUserInitials();
        const noteDate = note.date || note.createdAt || note.timestamp;
        const noteText = note.text || note.content || note.note;

        commentItem.innerHTML = `
          <div class="comment-header">
            <div class="comment-author">
              <div class="comment-avatar">${initials}</div>
              <div class="comment-author-info">
                <p class="comment-author-name">You</p>
                <span class="comment-date">${formatCommentDate(noteDate)}</span>
              </div>
            </div>
            <button type="button" class="comment-delete-button" aria-label="Delete note" data-comment-id="${
              note.id || note.noteId
            }">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
              </svg>
            </button>
          </div>
          <p class="comment-text">${escapeHtml(noteText)}</p>
        `;

        commentsList.appendChild(commentItem);
      });

      // Attach delete handlers
      commentsList
        .querySelectorAll(".comment-delete-button")
        .forEach((button) => {
          button.addEventListener("click", (e) => {
            e.stopPropagation();
            const commentId = button.dataset.commentId;
            deleteComment(fileId, commentId);
          });
        });
    }
  } catch (error) {
    console.error("Error rendering notes:", error);
    commentsList.innerHTML =
      '<div style="padding: 20px; text-align: center; color: var(--text-secondary);">Error loading notes</div>';
  }
}

// Add a new comment
async function addComment(fileId, text) {
  if (!text.trim()) {
    showToast("Note cannot be empty", "warning");
    return;
  }

  try {
    await addNote(fileId, text.trim());
    commentInput.value = "";
    await renderComments(fileId);
    showToast("Note added", "success");
  } catch (error) {
    console.error("Error adding note:", error);
    let errorMessage = "Unknown error";
    if (error instanceof APIError) {
      errorMessage = error.getUserMessage();
    } else if (error.message) {
      errorMessage = error.message;
    }
    showToast(`Failed to add note: ${errorMessage}`, "error");
  }
}

// Delete a comment
async function deleteComment(fileId, commentId) {
  if (!confirm("Are you sure you want to delete this note?")) {
    return;
  }

  try {
    await deleteNote(fileId, commentId);
    await renderComments(fileId);
    showToast("Note deleted", "success");
  } catch (error) {
    console.error("Error deleting note:", error);
    let errorMessage = "Unknown error";
    if (error instanceof APIError) {
      errorMessage = error.getUserMessage();
    } else if (error.message) {
      errorMessage = error.message;
    }
    showToast(`Failed to delete note: ${errorMessage}`, "error");
  }
}

// Open comments modal
async function openCommentsModal(galleryItem) {
  const fileId = galleryItem.dataset.fileId;
  const fileName = galleryItem.dataset.fileName;

  if (!fileId || !commentsModal || !commentsFileName) return;

  currentFileId = fileId;
  commentsFileName.textContent = fileName;
  commentsModal.style.display = "flex";
  document.body.style.overflow = "hidden";

  await renderComments(fileId);
  if (commentInput) commentInput.focus();
}

// Close comments modal
function closeCommentsModal() {
  if (commentsModal) {
    commentsModal.style.display = "none";
  }
  document.body.style.overflow = "";
  if (commentInput) commentInput.value = "";
  currentFileId = null;
}

// Comments modal initialization moved to initCommentsModal()

// Load statistics from API
async function loadStatistics() {
  const statsGrid = document.getElementById("stats-grid");
  const statsLoading = document.getElementById("stats-loading");
  const chartsContainer = document.getElementById("charts-container");

  if (!statsGrid) return;

  try {
    if (statsLoading) statsLoading.style.display = "block";

    const stats = await getStatistics();

    if (statsLoading) statsLoading.style.display = "none";

    // Render statistics cards
    const totalFiles = stats.totalFiles || stats.files || 0;
    const storageUsed = stats.storageUsed || stats.storage || "0 B";
    const collections = stats.collections || stats.collectionCount || 0;

    statsGrid.innerHTML = `
      <div class="stat-card">
        <div class="stat-header">
          <h3>Total Files</h3>
          <span class="stat-value">${totalFiles.toLocaleString()}</span>
        </div>
        <div class="stat-trend">
          <span class="trend-neutral">→</span>
          <span class="trend-text">Current count</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-header">
          <h3>Storage Used</h3>
          <span class="stat-value">${storageUsed}</span>
        </div>
        <div class="stat-trend">
          <span class="trend-neutral">→</span>
          <span class="trend-text">Current usage</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-header">
          <h3>Collections</h3>
          <span class="stat-value">${collections}</span>
        </div>
        <div class="stat-trend">
          <span class="trend-neutral">→</span>
          <span class="trend-text">Active collections</span>
        </div>
      </div>
    `;

    // Render charts if data available
    if (chartsContainer && stats.charts) {
      // Charts rendering can be added here based on backend response
      chartsContainer.innerHTML =
        '<p style="padding: 20px; text-align: center; color: var(--text-secondary);">Charts coming soon...</p>';
    }
  } catch (error) {
    console.error("Error loading statistics:", error);
    if (statsLoading) statsLoading.style.display = "none";
    if (statsGrid) {
      let errorMessage = "Error loading statistics";
      if (error instanceof APIError) {
        errorMessage = error.getUserMessage();
      } else if (error.message) {
        errorMessage = error.message;
      }
      statsGrid.innerHTML =
        `<div style="padding: 20px; text-align: center; color: var(--text-secondary);">${escapeHtml(errorMessage)}</div>`;
    }
    let errorMessage = "Failed to load statistics";
    if (error instanceof APIError) {
      errorMessage = error.getUserMessage();
    } else if (error.message) {
      errorMessage = error.message;
    }
    showToast(errorMessage, "error", 0, [{
      id: "retry",
      label: "Retry",
      handler: () => loadStatistics(),
    }]);
  }
}
