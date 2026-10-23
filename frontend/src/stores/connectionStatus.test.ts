import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { ConnectionStatus, type SSHTab } from "../types/ssh";
import {
  tabWithConnectionStatus,
  tabWithSessionConnectionStatus,
} from "./connectionStatus";

const baseTab = (): SSHTab => ({
  index: "t1",
  name: "host",
  sshInfo: {
    host: "1.2.3.4",
    port: 22,
    user: "u",
    linkID: "sess-1",
  },
  connectionStatus: ConnectionStatus.Connected,
});

describe("tabWithConnectionStatus", () => {
  it("Disconnected clears linkID and sets status", () => {
    const next = tabWithConnectionStatus(
      baseTab(),
      ConnectionStatus.Disconnected,
    );
    assert.equal(next.connectionStatus, ConnectionStatus.Disconnected);
    assert.equal(next.sshInfo?.linkID, undefined);
    assert.equal(next.sshInfo?.host, "1.2.3.4");
  });

  it("Connected/Connecting keep linkID", () => {
    const connecting = tabWithConnectionStatus(
      {
        ...baseTab(),
        connectionStatus: ConnectionStatus.Disconnected,
        sshInfo: { host: "h", port: 22, user: "u", linkID: "sess-2" },
      },
      ConnectionStatus.Connecting,
    );
    assert.equal(connecting.connectionStatus, ConnectionStatus.Connecting);
    assert.equal(connecting.sshInfo?.linkID, "sess-2");

    const connected = tabWithConnectionStatus(
      connecting,
      ConnectionStatus.Connected,
    );
    assert.equal(connected.connectionStatus, ConnectionStatus.Connected);
    assert.equal(connected.sshInfo?.linkID, "sess-2");
  });

  it("Disconnected without sshInfo is still safe", () => {
    const next = tabWithConnectionStatus(
      { index: "t2", name: "empty" },
      ConnectionStatus.Disconnected,
    );
    assert.equal(next.connectionStatus, ConnectionStatus.Disconnected);
    assert.equal(next.sshInfo, undefined);
  });
});

describe("tabWithSessionConnectionStatus", () => {
  it("Disconnected updates status but keeps linkID", () => {
    const next = tabWithSessionConnectionStatus(
      baseTab(),
      ConnectionStatus.Disconnected,
    );
    assert.ok(next);
    assert.equal(next.connectionStatus, ConnectionStatus.Disconnected);
    assert.equal(next.sshInfo?.linkID, "sess-1");
  });

  it("Disconnected is ignored while Connecting", () => {
    const connecting: SSHTab = {
      ...baseTab(),
      connectionStatus: ConnectionStatus.Connecting,
    };
    const next = tabWithSessionConnectionStatus(
      connecting,
      ConnectionStatus.Disconnected,
    );
    assert.equal(next, null);
  });

  it("Connected applies while Connecting", () => {
    const connecting: SSHTab = {
      ...baseTab(),
      connectionStatus: ConnectionStatus.Connecting,
    };
    const next = tabWithSessionConnectionStatus(
      connecting,
      ConnectionStatus.Connected,
    );
    assert.ok(next);
    assert.equal(next.connectionStatus, ConnectionStatus.Connected);
    assert.equal(next.sshInfo?.linkID, "sess-1");
  });
});
