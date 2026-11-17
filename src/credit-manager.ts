/**
 * Credit Manager for CycleTLS V2 Flow Control
 *
 * Tracks bytes received from the server and sends credit replenishment
 * packets when a threshold is reached. This implements backpressure
 * from the client side.
 */

/**
 * CreditManager tracks received bytes and triggers credit replenishment.
 */
export class CreditManager {
  private bytesReceived: number = 0;
  private threshold: number;
  private sendCredits: (credits: number) => void;
  private paused: boolean = false;

  /**
   * Create a new CreditManager.
   *
   * @param threshold - Number of bytes to receive before sending credits
   * @param sendCredits - Callback to send credits to the server
   */
  constructor(threshold: number, sendCredits: (credits: number) => void) {
    if (threshold <= 0) {
      throw new Error(`CreditManager threshold must be positive, got ${threshold}`);
    }
    this.threshold = threshold;
    this.sendCredits = sendCredits;
  }

  /**
   * Called when data is received from the server.
   * Will send credits when threshold is reached.
   *
   * @param bytes - Number of bytes received
   */
  onDataReceived(bytes: number): void {
    if (this.paused) {
      return;
    }

    this.bytesReceived += bytes;

    if (this.bytesReceived >= this.threshold) {
      const toSend = this.bytesReceived;
      this.bytesReceived = 0; // Reset before send to avoid accounting mismatch if send throws
      try {
        this.sendCredits(toSend);
      } catch {
        // sendCredits failure should not crash the client;
        // bytesReceived is already reset to prevent double-counting
      }
    }
  }

  /**
   * Pause credit sending. Useful for implementing client-side backpressure.
   */
  pause(): void {
    this.paused = true;
  }

  /**
   * Resume credit sending and flush any accumulated bytes.
   */
  resume(): void {
    this.paused = false;
    if (this.bytesReceived >= this.threshold) {
      const toSend = this.bytesReceived;
      this.bytesReceived = 0; // Reset before send to avoid accounting mismatch if send throws
      try {
        this.sendCredits(toSend);
      } catch {
        // sendCredits failure should not crash the client
      }
    }
  }

  /**
   * Flush any accumulated bytes as credits immediately.
   */
  flush(): void {
    if (!this.paused && this.bytesReceived > 0) {
      const toSend = this.bytesReceived;
      this.bytesReceived = 0; // Reset before send to avoid accounting mismatch if send throws
      try {
        this.sendCredits(toSend);
      } catch {
        // sendCredits failure should not crash the client;
        // bytesReceived is already reset to prevent double-counting
      }
    }
  }

  /**
   * Reset the manager state.
   */
  reset(): void {
    this.bytesReceived = 0;
    this.paused = false;
  }
}
