/**
 * Unit tests for ConfigService
 */

import { configService } from "../src/configService.js";

// Mock fetch globally
global.fetch = jest.fn();

describe("ConfigService", () => {
  beforeEach(() => {
    // Clear cache and reset state before each test
    configService.clearCache();
    fetch.mockClear();
  });

  describe("loadConfig", () => {
    it("should load and cache configuration successfully", async () => {
      const mockConfig = {
        auth_enabled: false,
        version: "1.0.0",
        features: {
          authentication: false,
          multi_tenant: true,
          async_ingestion: true,
          deduplication: true,
        },
      };

      fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockConfig,
      });

      const config = await configService.loadConfig();

      expect(config).toEqual(mockConfig);
      expect(fetch).toHaveBeenCalledWith("/api/config", {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      expect(configService.isAuthEnabled()).toBe(false);
    });

    it("should return cached config on subsequent calls", async () => {
      const mockConfig = {
        auth_enabled: true,
        version: "1.0.0",
        features: { authentication: true },
      };

      fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockConfig,
      });

      // First call
      await configService.loadConfig();
      fetch.mockClear();

      // Second call should use cache
      const config = await configService.loadConfig();

      expect(config).toEqual(mockConfig);
      expect(fetch).not.toHaveBeenCalled();
    });

    it("should handle fetch errors gracefully with default config", async () => {
      fetch.mockRejectedValueOnce(new Error("Network error"));

      const config = await configService.loadConfig();

      expect(config.auth_enabled).toBe(false);
      expect(config.version).toBe("unknown");
      expect(config.features.authentication).toBe(false);
    });

    it("should handle non-OK responses with default config", async () => {
      fetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      const config = await configService.loadConfig();

      expect(config.auth_enabled).toBe(false);
      expect(config.version).toBe("unknown");
    });

    it("should wait for ongoing load requests", async () => {
      let resolveFetch;
      const fetchPromise = new Promise((resolve) => {
        resolveFetch = resolve;
      });

      fetch.mockReturnValueOnce(fetchPromise);

      const promise1 = configService.loadConfig();
      const promise2 = configService.loadConfig(); // Should wait for first

      resolveFetch({
        ok: true,
        json: async () => ({
          auth_enabled: true,
          version: "1.0.0",
          features: {},
        }),
      });

      const [config1, config2] = await Promise.all([promise1, promise2]);

      expect(config1).toEqual(config2);
      expect(fetch).toHaveBeenCalledTimes(1);
    });
  });

  describe("isAuthEnabled", () => {
    it("should return false when config is not loaded", () => {
      configService.clearCache();
      expect(configService.isAuthEnabled()).toBe(false);
    });

    it("should return true when auth is enabled", async () => {
      fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          auth_enabled: true,
          version: "1.0.0",
          features: { authentication: true },
        }),
      });

      await configService.loadConfig();
      expect(configService.isAuthEnabled()).toBe(true);
    });

    it("should return false when auth is disabled", async () => {
      fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          auth_enabled: false,
          version: "1.0.0",
          features: { authentication: false },
        }),
      });

      await configService.loadConfig();
      expect(configService.isAuthEnabled()).toBe(false);
    });
  });

  describe("getVersion", () => {
    it("should return version from config", async () => {
      fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          auth_enabled: false,
          version: "2.0.0",
          features: {},
        }),
      });

      await configService.loadConfig();
      expect(configService.getVersion()).toBe("2.0.0");
    });

    it("should return 'unknown' when config is not loaded", () => {
      configService.clearCache();
      expect(configService.getVersion()).toBe("unknown");
    });
  });

  describe("hasFeature", () => {
    it("should return true for enabled features", async () => {
      fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          auth_enabled: false,
          version: "1.0.0",
          features: {
            multi_tenant: true,
            deduplication: true,
          },
        }),
      });

      await configService.loadConfig();
      expect(configService.hasFeature("multi_tenant")).toBe(true);
      expect(configService.hasFeature("deduplication")).toBe(true);
    });

    it("should return false for disabled features", async () => {
      fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          auth_enabled: false,
          version: "1.0.0",
          features: {
            async_ingestion: false,
          },
        }),
      });

      await configService.loadConfig();
      expect(configService.hasFeature("async_ingestion")).toBe(false);
    });

    it("should return false for non-existent features", async () => {
      fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          auth_enabled: false,
          version: "1.0.0",
          features: {},
        }),
      });

      await configService.loadConfig();
      expect(configService.hasFeature("non_existent")).toBe(false);
    });
  });

  describe("clearCache", () => {
    it("should clear cached configuration", async () => {
      fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          auth_enabled: true,
          version: "1.0.0",
          features: {},
        }),
      });

      await configService.loadConfig();
      expect(configService.isAuthEnabled()).toBe(true);

      configService.clearCache();
      expect(configService.isAuthEnabled()).toBe(false);
    });
  });
});


