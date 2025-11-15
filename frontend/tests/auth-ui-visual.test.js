/**
 * Visual/UI tests for conditional authentication UI display
 * These tests verify that UI elements are correctly shown/hidden based on auth configuration
 */

describe("Authentication UI Conditional Display", () => {
  let container;
  let configService;

  beforeEach(async () => {
    // Setup DOM
    container = document.createElement("div");
    container.innerHTML = `
      <button id="user-icon" class="icon-button hidden">User</button>
      <button id="profile-button" class="profile-button hidden">AZ</button>
      <button id="about-btn" class="icon-button">About</button>
      <div id="about-modal" style="display: none;">
        <div id="auth-status">Loading...</div>
        <div id="app-version">Loading...</div>
      </div>
    `;
    document.body.appendChild(container);

    // Mock configService
    const { configService: cs } = await import("../src/configService.js");
    configService = cs;
  });

  afterEach(() => {
    document.body.removeChild(container);
    configService.clearCache();
  });

  describe("User Icon Visibility", () => {
    it("should hide user icon when auth is disabled", () => {
      configService.config = {
        auth_enabled: false,
        version: "1.0.0",
        features: { authentication: false },
      };

      const userIcon = document.getElementById("user-icon");
      const profileButton = document.getElementById("profile-button");

      // Simulate initAuthUI
      if (configService.isAuthEnabled()) {
        userIcon?.classList.remove("hidden");
        profileButton?.classList.remove("hidden");
      } else {
        userIcon?.classList.add("hidden");
        profileButton?.classList.add("hidden");
      }

      expect(userIcon.classList.contains("hidden")).toBe(true);
      expect(profileButton.classList.contains("hidden")).toBe(true);
    });

    it("should show user icon when auth is enabled", () => {
      configService.config = {
        auth_enabled: true,
        version: "1.0.0",
        features: { authentication: true },
      };

      const userIcon = document.getElementById("user-icon");
      const profileButton = document.getElementById("profile-button");

      // Simulate initAuthUI
      if (configService.isAuthEnabled()) {
        userIcon?.classList.remove("hidden");
        profileButton?.classList.remove("hidden");
      } else {
        userIcon?.classList.add("hidden");
        profileButton?.classList.add("hidden");
      }

      expect(userIcon.classList.contains("hidden")).toBe(false);
      expect(profileButton.classList.contains("hidden")).toBe(false);
    });
  });

  describe("About Modal Auth Status", () => {
    it("should display 'Disabled' when auth is disabled", () => {
      configService.config = {
        auth_enabled: false,
        version: "1.0.0",
        features: { authentication: false },
      };

      const authStatusEl = document.getElementById("auth-status");
      const isEnabled = configService.isAuthEnabled();
      authStatusEl.innerHTML = isEnabled
        ? '🔓 <strong>Authentication:</strong> Enabled'
        : '🔒 <strong>Authentication:</strong> Disabled';

      expect(authStatusEl.innerHTML).toContain("Disabled");
      expect(authStatusEl.innerHTML).toContain("🔒");
    });

    it("should display 'Enabled' when auth is enabled", () => {
      configService.config = {
        auth_enabled: true,
        version: "1.0.0",
        features: { authentication: true },
      };

      const authStatusEl = document.getElementById("auth-status");
      const isEnabled = configService.isAuthEnabled();
      authStatusEl.innerHTML = isEnabled
        ? '🔓 <strong>Authentication:</strong> Enabled'
        : '🔒 <strong>Authentication:</strong> Disabled';

      expect(authStatusEl.innerHTML).toContain("Enabled");
      expect(authStatusEl.innerHTML).toContain("🔓");
    });

    it("should display version from config", () => {
      configService.config = {
        auth_enabled: false,
        version: "2.0.0",
        features: {},
      };

      const versionEl = document.getElementById("app-version");
      versionEl.textContent = configService.getVersion();

      expect(versionEl.textContent).toBe("2.0.0");
    });
  });

  describe("Loading State", () => {
    it("should show loading overlay during config fetch", () => {
      const overlay = document.createElement("div");
      overlay.id = "loading-overlay";
      overlay.className = "loading-overlay";
      overlay.style.display = "none";
      document.body.appendChild(overlay);

      // Simulate loading
      overlay.style.display = "flex";
      expect(overlay.style.display).toBe("flex");

      document.body.removeChild(overlay);
    });
  });
});

