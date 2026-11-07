import type { Config } from "../../bindings/github.com/ilaziness/vela/services";
import { Provider } from "../../bindings/github.com/ilaziness/vela/internal/sync/models";

type SyncConfig = Config["Sync"];

export function isSyncConfigured(cfg: SyncConfig | null | undefined): boolean {
  if (!cfg?.provider) {
    return false;
  }
  switch (cfg.provider) {
    case Provider.ProviderSelfHosted:
      return Boolean(
        cfg.selfhosted?.serverUrl &&
          cfg.selfhosted?.syncId &&
          (cfg.selfhosted?.userKey || cfg.selfhosted?.userKeyEncrypted),
      );
    case Provider.ProviderWebDAV:
      return Boolean(
        cfg.webdav?.url &&
          cfg.webdav?.username &&
          cfg.webdav?.remotePath &&
          (cfg.webdav?.password || cfg.webdav?.passwordEncrypted),
      );
    case Provider.ProviderS3:
      return Boolean(
        cfg.s3?.endpoint &&
          cfg.s3?.bucket &&
          cfg.s3?.accessKey &&
          (cfg.s3?.secretKey || cfg.s3?.secretKeyEncrypted),
      );
    default:
      return false;
  }
}
