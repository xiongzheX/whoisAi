(function (root) {
  'use strict';
  // Use an absolute deadline so background-tab throttling cannot extend a turn.
  class TurnClock {
    constructor(now = Date.now) { this.now = now; this.deadline = null; }
    start(seconds = 20) { this.deadline = this.now() + seconds * 1000; }
    stop() { this.deadline = null; }
    remaining() { return this.deadline === null ? 0 : Math.max(0, this.deadline - this.now()); }
    expired() { return this.deadline !== null && this.remaining() === 0; }
    consumeExpiry() {
      if (!this.expired()) return false;
      this.stop();
      return true;
    }
  }
  if (typeof module !== 'undefined' && module.exports) module.exports = TurnClock;
  else root.PokerTurnClock = TurnClock;
})(typeof window !== 'undefined' ? window : globalThis);
