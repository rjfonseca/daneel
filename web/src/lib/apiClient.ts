import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { GraphService } from '../gen/api/v1/graph_connect';

const apiTarget = import.meta.env.VITE_API_URL || "http://localhost:8088";

export const transport = createConnectTransport({ 
  baseUrl: import.meta.env.DEV ? apiTarget : window.location.origin 
});

export const client = createPromiseClient(GraphService, transport);
