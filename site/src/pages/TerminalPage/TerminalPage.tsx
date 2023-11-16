import { type Interpolation, type Theme, useTheme } from "@emotion/react";
import { type FC, useCallback, useEffect, useRef, useState } from "react";
import { Helmet } from "react-helmet-async";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { colors } from "theme/colors";
import { v4 as uuidv4 } from "uuid";
import * as XTerm from "xterm";
import { WebglAddon } from "xterm-addon-webgl";
import { CanvasAddon } from "xterm-addon-canvas";
import { FitAddon } from "xterm-addon-fit";
import { WebLinksAddon } from "xterm-addon-web-links";
import { Unicode11Addon } from "xterm-addon-unicode11";
import "xterm/css/xterm.css";
import { MONOSPACE_FONT_FAMILY } from "theme/constants";
import { pageTitle } from "utils/page";
import { useProxy } from "contexts/ProxyContext";
import Box from "@mui/material/Box";
import { useDashboard } from "components/Dashboard/DashboardProvider";
import type { Region } from "api/typesGenerated";
import { getLatencyColor } from "utils/latency";
import { ProxyStatusLatency } from "components/ProxyStatusLatency/ProxyStatusLatency";
import { openMaybePortForwardedURL } from "utils/portForward";
import { terminalWebsocketUrl } from "utils/terminal";
import { getMatchingAgentOrFirst } from "utils/workspace";
import {
  DisconnectedAlert,
  ErrorScriptAlert,
  LoadedScriptsAlert,
  LoadingScriptsAlert,
} from "./TerminalAlerts";
import { useQuery } from "react-query";
import { deploymentConfig } from "api/queries/deployment";
import { workspaceByOwnerAndName } from "api/queries/workspaces";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "components/Popover/Popover";

export const Language = {
  workspaceErrorMessagePrefix: "Unable to fetch workspace: ",
  workspaceAgentErrorMessagePrefix: "Unable to fetch workspace agent: ",
  websocketErrorMessagePrefix: "WebSocket failed: ",
};

const TerminalPage: FC = () => {
  const navigate = useNavigate();
  const { proxy } = useProxy();
  const params = useParams() as { username: string; workspace: string };
  const username = params.username.replace("@", "");
  const xtermRef = useRef<HTMLDivElement>(null);
  const [terminal, setTerminal] = useState<XTerm.Terminal | null>(null);
  const [terminalState, setTerminalState] = useState<
    "connected" | "disconnected" | "initializing"
  >("initializing");
  const [searchParams] = useSearchParams();
  // The reconnection token is a unique token that identifies
  // a terminal session. It's generated by the client to reduce
  // a round-trip, and must be a UUIDv4.
  const reconnectionToken = searchParams.get("reconnect") ?? uuidv4();
  const command = searchParams.get("command") || undefined;
  // The workspace name is in the format:
  // <workspace name>[.<agent name>]
  const workspaceNameParts = params.workspace?.split(".");
  const workspace = useQuery(
    workspaceByOwnerAndName(username, workspaceNameParts?.[0]),
  );
  const workspaceAgent = workspace.data
    ? getMatchingAgentOrFirst(workspace.data, workspaceNameParts?.[1])
    : undefined;
  const dashboard = useDashboard();
  const proxyContext = useProxy();
  const selectedProxy = proxyContext.proxy.proxy;
  const latency = selectedProxy
    ? proxyContext.proxyLatencies[selectedProxy.id]
    : undefined;

  const lifecycleState = workspaceAgent?.lifecycle_state;
  const prevLifecycleState = useRef(lifecycleState);
  useEffect(() => {
    prevLifecycleState.current = lifecycleState;
  }, [lifecycleState]);

  const config = useQuery(deploymentConfig());
  const renderer = config.data?.config.web_terminal_renderer;

  // handleWebLink handles opening of URLs in the terminal!
  const handleWebLink = useCallback(
    (uri: string) => {
      openMaybePortForwardedURL(
        uri,
        proxy.preferredWildcardHostname,
        workspaceAgent?.name,
        workspace.data?.name,
        username,
      );
    },
    [workspaceAgent, workspace.data, username, proxy.preferredWildcardHostname],
  );
  const handleWebLinkRef = useRef(handleWebLink);
  useEffect(() => {
    handleWebLinkRef.current = handleWebLink;
  }, [handleWebLink]);

  // Create the terminal!
  useEffect(() => {
    if (!xtermRef.current || config.isLoading) {
      return;
    }
    const terminal = new XTerm.Terminal({
      allowProposedApi: true,
      allowTransparency: true,
      disableStdin: false,
      fontFamily: MONOSPACE_FONT_FAMILY,
      fontSize: 16,
      theme: {
        background: colors.gray[16],
      },
    });
    if (renderer === "webgl") {
      terminal.loadAddon(new WebglAddon());
    } else if (renderer === "canvas") {
      terminal.loadAddon(new CanvasAddon());
    }
    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.loadAddon(new Unicode11Addon());
    terminal.unicode.activeVersion = "11";
    terminal.loadAddon(
      new WebLinksAddon((_, uri) => {
        handleWebLinkRef.current(uri);
      }),
    );

    terminal.open(xtermRef.current);

    // We have to fit twice here. It's unknown why, but the first fit will
    // overflow slightly in some scenarios. Applying a second fit resolves this.
    fitAddon.fit();
    fitAddon.fit();

    // This will trigger a resize event on the terminal.
    const listener = () => fitAddon.fit();
    window.addEventListener("resize", listener);

    // Terminal is correctly sized and is ready to be used.
    setTerminal(terminal);

    return () => {
      window.removeEventListener("resize", listener);
      terminal.dispose();
    };
  }, [renderer, config.isLoading, xtermRef, handleWebLinkRef]);

  // Updates the reconnection token into the URL if necessary.
  useEffect(() => {
    if (searchParams.get("reconnect") === reconnectionToken) {
      return;
    }
    searchParams.set("reconnect", reconnectionToken);
    navigate(
      {
        search: searchParams.toString(),
      },
      {
        replace: true,
      },
    );
  }, [searchParams, navigate, reconnectionToken]);

  // Hook up the terminal through a web socket.
  useEffect(() => {
    if (!terminal) {
      return;
    }

    // The terminal should be cleared on each reconnect
    // because all data is re-rendered from the backend.
    terminal.clear();

    // Focusing on connection allows users to reload the page and start
    // typing immediately.
    terminal.focus();

    // Disable input while we connect.
    terminal.options.disableStdin = true;

    // Show a message if we failed to find the workspace or agent.
    if (workspace.isLoading) {
      return;
    } else if (workspace.error instanceof Error) {
      terminal.writeln(
        Language.workspaceErrorMessagePrefix + workspace.error.message,
      );
      return;
    } else if (!workspaceAgent) {
      terminal.writeln(
        Language.workspaceAgentErrorMessagePrefix + "no agent found with ID",
      );
      return;
    }

    // Hook up terminal events to the websocket.
    let websocket: WebSocket | null;
    const disposers = [
      terminal.onData((data) => {
        websocket?.send(
          new TextEncoder().encode(JSON.stringify({ data: data })),
        );
      }),
      terminal.onResize((event) => {
        websocket?.send(
          new TextEncoder().encode(
            JSON.stringify({
              height: event.rows,
              width: event.cols,
            }),
          ),
        );
      }),
    ];

    let disposed = false;

    // Open the web socket and hook it up to the terminal.
    terminalWebsocketUrl(
      proxy.preferredPathAppURL,
      reconnectionToken,
      workspaceAgent.id,
      command,
      terminal.rows,
      terminal.cols,
    )
      .then((url) => {
        if (disposed) {
          return; // Unmounted while we waited for the async call.
        }
        websocket = new WebSocket(url);
        websocket.binaryType = "arraybuffer";
        websocket.addEventListener("open", () => {
          // Now that we are connected, allow user input.
          terminal.options = {
            disableStdin: false,
            windowsMode: workspaceAgent?.operating_system === "windows",
          };
          // Send the initial size.
          websocket?.send(
            new TextEncoder().encode(
              JSON.stringify({
                height: terminal.rows,
                width: terminal.cols,
              }),
            ),
          );
          setTerminalState("connected");
        });
        websocket.addEventListener("error", () => {
          terminal.options.disableStdin = true;
          terminal.writeln(
            Language.websocketErrorMessagePrefix + "socket errored",
          );
          setTerminalState("disconnected");
        });
        websocket.addEventListener("close", () => {
          terminal.options.disableStdin = true;
          setTerminalState("disconnected");
        });
        websocket.addEventListener("message", (event) => {
          if (typeof event.data === "string") {
            // This exclusively occurs when testing.
            // "jest-websocket-mock" doesn't support ArrayBuffer.
            terminal.write(event.data);
          } else {
            terminal.write(new Uint8Array(event.data));
          }
        });
      })
      .catch((error) => {
        if (disposed) {
          return; // Unmounted while we waited for the async call.
        }
        terminal.writeln(Language.websocketErrorMessagePrefix + error.message);
        setTerminalState("disconnected");
      });

    return () => {
      disposed = true; // Could use AbortController instead?
      disposers.forEach((d) => d.dispose());
      websocket?.close(1000);
    };
  }, [
    command,
    proxy.preferredPathAppURL,
    reconnectionToken,
    terminal,
    workspace.isLoading,
    workspace.error,
    workspaceAgent,
  ]);

  return (
    <>
      <Helmet>
        <title>
          {workspace.data
            ? pageTitle(
                `Terminal · ${workspace.data.owner_name}/${workspace.data.name}`,
              )
            : ""}
        </title>
      </Helmet>
      <Box display="flex" flexDirection="column" height="100vh">
        {lifecycleState === "start_error" && <ErrorScriptAlert />}
        {lifecycleState === "starting" && <LoadingScriptsAlert />}
        {lifecycleState === "ready" &&
          prevLifecycleState.current === "starting" && <LoadedScriptsAlert />}
        {terminalState === "disconnected" && <DisconnectedAlert />}
        <div css={styles.terminal} ref={xtermRef} data-testid="terminal" />
        {dashboard.experiments.includes("moons") &&
          selectedProxy &&
          latency && (
            <BottomBar proxy={selectedProxy} latency={latency.latencyMS} />
          )}
      </Box>
    </>
  );
};

const BottomBar = ({ proxy, latency }: { proxy: Region; latency?: number }) => {
  const theme = useTheme();
  const color = getLatencyColor(theme, latency);

  return (
    <Box
      sx={{
        padding: "0 16px",
        background: (theme) => theme.palette.background.paper,
        display: "flex",
        alignItems: "center",
        justifyContent: "flex-end",
        fontSize: 12,
        borderTop: (theme) => `1px solid ${theme.palette.divider}`,
      }}
    >
      <Popover mode="hover">
        <PopoverTrigger>
          <Box
            component="button"
            aria-label="Terminal latency"
            aria-haspopup="true"
            css={{
              background: "none",
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              gap: 8,
              border: 0,
              padding: 8,
            }}
          >
            <Box
              sx={{
                height: 6,
                width: 6,
                backgroundColor: color,
                border: 0,
                borderRadius: 9999,
              }}
            />
            <ProxyStatusLatency latency={latency} />
          </Box>
        </PopoverTrigger>
        <PopoverContent
          id="latency-popover"
          disableRestoreFocus
          sx={{
            pointerEvents: "none",
            "& .MuiPaper-root": {
              padding: "8px 16px",
            },
          }}
          anchorOrigin={{
            vertical: "top",
            horizontal: "right",
          }}
          transformOrigin={{
            vertical: "bottom",
            horizontal: "right",
          }}
        >
          <Box
            sx={{
              fontSize: 13,
              color: (theme) => theme.palette.text.secondary,
              fontWeight: 500,
            }}
          >
            Selected proxy
          </Box>
          <Box
            sx={{ fontSize: 14, display: "flex", gap: 3, alignItems: "center" }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Box width={12} height={12} lineHeight={0}>
                <Box
                  component="img"
                  src={proxy.icon_url}
                  alt=""
                  sx={{ objectFit: "contain" }}
                  width="100%"
                  height="100%"
                />
              </Box>
              {proxy.display_name}
            </Box>
            <ProxyStatusLatency latency={latency} />
          </Box>
        </PopoverContent>
      </Popover>
    </Box>
  );
};

const styles = {
  terminal: (theme) => ({
    width: "100vw",
    overflow: "hidden",
    backgroundColor: theme.palette.background.paper,
    flex: 1,
    // These styles attempt to mimic the VS Code scrollbar.
    "& .xterm": {
      padding: 4,
      width: "100vw",
      height: "100vh",
    },
    "& .xterm-viewport": {
      // This is required to force full-width on the terminal.
      // Otherwise there's a small white bar to the right of the scrollbar.
      width: "auto !important",
    },
    "& .xterm-viewport::-webkit-scrollbar": {
      width: "10px",
    },
    "& .xterm-viewport::-webkit-scrollbar-track": {
      backgroundColor: "inherit",
    },
    "& .xterm-viewport::-webkit-scrollbar-thumb": {
      minHeight: 20,
      backgroundColor: "rgba(255, 255, 255, 0.18)",
    },
  }),
} satisfies Record<string, Interpolation<Theme>>;

export default TerminalPage;
