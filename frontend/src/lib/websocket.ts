// WebSocket client for real-time updates

export type EventType = 'block:new' | 'tx:new' | 'address:activity' | 'price:update' | 'sync:status';

export interface WSMessage {
  type: 'event' | 'success' | 'error';
  topic?: string;
  action?: string;
  data?: unknown;
  message?: string;
}

export interface WSClientOptions {
  url?: string;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  onOpen?: () => void;
  onClose?: () => void;
  onError?: (error: Event) => void;
  onMessage?: (message: WSMessage) => void;
}

export type TopicHandler = (data: unknown) => void;

const DEFAULT_OPTIONS: WSClientOptions = {
  reconnectInterval: 3000,
  maxReconnectAttempts: 10,
};

export class WSClient {
  private ws: WebSocket | null = null;
  private options: WSClientOptions;
  private reconnectAttempts = 0;
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private handlers: Map<string, Set<TopicHandler>> = new Map();
  private pendingSubscriptions: Set<string> = new Set();
  private isConnected = false;

  constructor(options: WSClientOptions = {}) {
    this.options = { ...DEFAULT_OPTIONS, ...options };
  }

  /**
   * Connect to the WebSocket server
   */
  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return;
    }

    const url = this.options.url || this.getDefaultWSUrl();
    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      this.isConnected = true;
      this.reconnectAttempts = 0;

      // Re-subscribe to topics
      if (this.pendingSubscriptions.size > 0) {
        this.send({
          action: 'subscribe',
          topics: Array.from(this.pendingSubscriptions),
        });
      }

      this.options.onOpen?.();
    };

    this.ws.onclose = () => {
      this.isConnected = false;
      this.options.onClose?.();
      this.scheduleReconnect();
    };

    this.ws.onerror = (error) => {
      this.options.onError?.(error);
    };

    this.ws.onmessage = (event) => {
      try {
        const message: WSMessage = JSON.parse(event.data);
        this.handleMessage(message);
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
      }
    };
  }

  /**
   * Disconnect from the WebSocket server
   */
  disconnect(): void {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }

    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }

    this.isConnected = false;
  }

  /**
   * Subscribe to a topic
   */
  subscribe(topic: string, handler: TopicHandler): () => void {
    // Add handler
    if (!this.handlers.has(topic)) {
      this.handlers.set(topic, new Set());
    }
    this.handlers.get(topic)!.add(handler);

    // Track subscription
    this.pendingSubscriptions.add(topic);

    // Send subscription if connected
    if (this.isConnected) {
      this.send({ action: 'subscribe', topics: [topic] });
    }

    // Return unsubscribe function
    return () => this.unsubscribe(topic, handler);
  }

  /**
   * Unsubscribe from a topic
   */
  unsubscribe(topic: string, handler?: TopicHandler): void {
    if (handler) {
      this.handlers.get(topic)?.delete(handler);
      if (this.handlers.get(topic)?.size === 0) {
        this.handlers.delete(topic);
        this.pendingSubscriptions.delete(topic);
        if (this.isConnected) {
          this.send({ action: 'unsubscribe', topics: [topic] });
        }
      }
    } else {
      this.handlers.delete(topic);
      this.pendingSubscriptions.delete(topic);
      if (this.isConnected) {
        this.send({ action: 'unsubscribe', topics: [topic] });
      }
    }
  }

  /**
   * Subscribe to new blocks
   */
  onNewBlock(handler: TopicHandler): () => void {
    return this.subscribe('blocks', handler);
  }

  /**
   * Subscribe to new transactions
   */
  onNewTransaction(handler: TopicHandler): () => void {
    return this.subscribe('transactions', handler);
  }

  /**
   * Subscribe to price updates
   */
  onPriceUpdate(handler: TopicHandler): () => void {
    return this.subscribe('price', handler);
  }

  /**
   * Subscribe to sync status updates
   */
  onSyncStatus(handler: TopicHandler): () => void {
    return this.subscribe('sync', handler);
  }

  /**
   * Subscribe to address activity
   */
  onAddressActivity(address: string, handler: TopicHandler): () => void {
    return this.subscribe(`address:${address}`, handler);
  }

  /**
   * Check if connected
   */
  get connected(): boolean {
    return this.isConnected;
  }

  private getDefaultWSUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    return `${protocol}//${host}/ws`;
  }

  private send(data: Record<string, unknown>): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  private handleMessage(message: WSMessage): void {
    this.options.onMessage?.(message);

    if (message.type === 'event' && message.topic) {
      const handlers = this.handlers.get(message.topic);
      if (handlers) {
        handlers.forEach((handler) => {
          try {
            handler(message.data);
          } catch (e) {
            console.error('Error in WebSocket handler:', e);
          }
        });
      }
    }
  }

  private scheduleReconnect(): void {
    if (
      this.options.maxReconnectAttempts &&
      this.reconnectAttempts >= this.options.maxReconnectAttempts
    ) {
      console.error('Max reconnect attempts reached');
      return;
    }

    this.reconnectTimeout = setTimeout(() => {
      this.reconnectAttempts++;
      this.connect();
    }, this.options.reconnectInterval);
  }
}

// Singleton instance for convenience
let defaultClient: WSClient | null = null;

export function getWSClient(options?: WSClientOptions): WSClient {
  if (!defaultClient) {
    defaultClient = new WSClient(options);
  }
  return defaultClient;
}

// React hook for using WebSocket subscriptions
export function useWebSocket(
  topic: string,
  handler: TopicHandler,
  options?: WSClientOptions
): void {
  // This is a placeholder - actual implementation would use React's useEffect
  const client = getWSClient(options);
  client.connect();
  client.subscribe(topic, handler);
}
