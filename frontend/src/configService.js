// Configuration service for fetching and caching application configuration
class ConfigService {
  constructor() {
    this.config = null;
    this.loading = false;
    this.error = null;
    this.loadPromise = null;
  }

  /**
   * Load configuration from the backend API
   * @returns {Promise<Object>} Configuration object
   */
  async loadConfig() {
    // Return cached config if available
    if (this.config) {
      return this.config;
    }

    // If already loading, wait for the existing promise
    if (this.loading && this.loadPromise) {
      return this.loadPromise;
    }

    this.loading = true;
    this.error = null;

    this.loadPromise = (async () => {
      try {
        const response = await fetch("/api/config", {
          method: "GET",
          headers: { Accept: "application/json" },
        });

        if (!response.ok) {
          throw new Error(`Config load failed: ${response.status}`);
        }

        this.config = await response.json();
        this.loading = false;
        return this.config;
      } catch (error) {
        console.error("Failed to load config:", error);
        this.error = error;
        this.loading = false;

        // Default config when backend unavailable
        this.config = {
          auth_enabled: false,
          version: "unknown",
          features: {
            authentication: false,
            multi_tenant: true,
            async_ingestion: true,
            deduplication: true,
          },
        };

        return this.config;
      } finally {
        this.loadPromise = null;
      }
    })();

    return this.loadPromise;
  }

  /**
   * Check if authentication is enabled
   * @returns {boolean}
   */
  isAuthEnabled() {
    return this.config?.auth_enabled ?? false;
  }

  /**
   * Get the application version
   * @returns {string}
   */
  getVersion() {
    return this.config?.version ?? "unknown";
  }

  /**
   * Check if a specific feature is enabled
   * @param {string} featureName - Name of the feature
   * @returns {boolean}
   */
  hasFeature(featureName) {
    return this.config?.features?.[featureName] ?? false;
  }

  /**
   * Get all features
   * @returns {Object}
   */
  getFeatures() {
    return this.config?.features ?? {};
  }

  /**
   * Clear cached configuration (useful for testing or refresh)
   */
  clearCache() {
    this.config = null;
    this.error = null;
    this.loadPromise = null;
  }
}

// Singleton instance
export const configService = new ConfigService();


