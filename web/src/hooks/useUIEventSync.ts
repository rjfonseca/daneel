import { useState, useEffect, useCallback, useRef } from 'react';
import { client } from '../lib/apiClient';
import { UIActionType, SubscribeUIEventsRequest } from '../gen/api/v1/graph_pb';
import { addLocalPinnedAnchor, removeLocalPinnedAnchor } from './usePinnedAnchors';

interface UseUIEventSyncProps {
  setSelectedNodeId: (id: string | null) => void;
  setIsDrawerOpen: (isOpen: boolean) => void;
  setIsConstellationIsolated: (isIsolated: boolean) => void;
  setFocusedSector: (sector: string | null) => void;
  reloadGalaxy: () => void;
  selectedNodeId: string | null;
  focusedSector: string | null;
}

export function useUIEventSync({
  setSelectedNodeId,
  setIsDrawerOpen,
  setIsConstellationIsolated,
  setFocusedSector,
  reloadGalaxy,
  selectedNodeId,
  focusedSector,
}: UseUIEventSyncProps) {
  const [toastNotification, setToastNotification] = useState<{ title: string; message: string; id: number } | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const abortControllerRef = useRef<AbortController | null>(null);
  const handlersRef = useRef({
    setSelectedNodeId, setIsDrawerOpen, setIsConstellationIsolated, setFocusedSector, reloadGalaxy,
  });
  handlersRef.current = {
    setSelectedNodeId, setIsDrawerOpen, setIsConstellationIsolated, setFocusedSector, reloadGalaxy,
  };

  const clearToast = useCallback(() => {
    setToastNotification(null);
  }, []);

  useEffect(() => {
    let reconnectTimeout: ReturnType<typeof setTimeout>;
    let isMounted = true;
    let attempt = 0;

    const connect = async () => {
      try {
        const controller = new AbortController();
        abortControllerRef.current = controller;
        
        const request = new SubscribeUIEventsRequest();
        const responseStream = client.subscribeUIEvents(request, { signal: controller.signal });
        
        setIsConnected(true);

        for await (const event of responseStream) {
          if (!isMounted) break;
          attempt = 0;
          const handlers = handlersRef.current;

          switch (event.action) {
            case UIActionType.ACTION_OPEN_NOTE:
              if (event.targetPath) {
                handlers.setSelectedNodeId(event.targetPath);
                handlers.setIsDrawerOpen(true);
                handlers.setIsConstellationIsolated(true);
              }
              break;
            case UIActionType.ACTION_FOCUS_DOMAIN:
            case UIActionType.ACTION_FOCUS_SECTOR:
              handlers.setFocusedSector(event.targetPath || null);
              break;
            case UIActionType.ACTION_FLASH_NOTIFICATION: {
              let title = 'System Notification';
              let message = event.targetPath;
              if (event.payloadJson) {
                try {
                  const parsed = JSON.parse(event.payloadJson);
                  if (parsed.title) title = parsed.title;
                  if (parsed.message) message = parsed.message;
                } catch {
                  message = event.payloadJson;
                }
              }
              setToastNotification({
                title,
                message,
                id: Date.now()
              });
              break;
            }
            case UIActionType.ACTION_GALAXY_RELOAD:
              handlers.reloadGalaxy();
              break;
            case UIActionType.ACTION_PIN_NOTE: {
              if (event.targetPath) {
                let label: string | undefined;
                let icon: string | undefined;
                if (event.payloadJson) {
                  try {
                    const parsed = JSON.parse(event.payloadJson);
                    if (parsed.label) label = parsed.label;
                    if (parsed.icon) icon = parsed.icon;
                  } catch {
                    // ignore JSON parse error
                  }
                }
                addLocalPinnedAnchor({ path: event.targetPath, label, icon });
                const displayLabel = label || event.targetPath.split('/').pop()?.replace(/\.md$/, '') || event.targetPath;
                setToastNotification({
                  title: 'Seldon Vault Anchors',
                  message: `Pinned to hotbar: ${displayLabel}`,
                  id: Date.now(),
                });
              }
              break;
            }
            case UIActionType.ACTION_UNPIN_NOTE: {
              if (event.targetPath) {
                removeLocalPinnedAnchor(event.targetPath);
                const displayLabel = event.targetPath.split('/').pop()?.replace(/\.md$/, '') || event.targetPath;
                setToastNotification({
                  title: 'Seldon Vault Anchors',
                  message: `Unpinned from hotbar: ${displayLabel}`,
                  id: Date.now(),
                });
              }
              break;
            }
            default:
              break;
          }
        }
      } catch (err: unknown) {
        if (isMounted && !abortControllerRef.current?.signal.aborted &&
            !(err instanceof Error && err.name === 'AbortError')) {
          setIsConnected(false);
          if (isMounted) {
            const delay = Math.min(1000 * Math.pow(2, attempt), 30000);
            attempt++;
            reconnectTimeout = setTimeout(connect, delay);
          }
        }
      } finally {
        if (isMounted) {
          setIsConnected(false);
        }
      }
    };

    connect();

    return () => {
      isMounted = false;
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
      clearTimeout(reconnectTimeout);
    };
  }, []);

  useEffect(() => {
    const reportContext = async () => {
      try {
        await client.reportActiveContext({
          surface: "web",
          activeNotePath: selectedNodeId || "",
          focusedDomain: focusedSector || "",
          selectedText: window.getSelection()?.toString() || ""
        });
      } catch {
        // Silent failure for context reporting
      }
    };

    const handler = setTimeout(() => {
      reportContext();
    }, 500);

    return () => clearTimeout(handler);
  }, [selectedNodeId, focusedSector]);

  return { toastNotification, clearToast, isConnected };
}
