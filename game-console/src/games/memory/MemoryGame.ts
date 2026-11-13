import { Game } from '../../core/Game';
import { MatchInstance } from '../../core/MatchInstance';
import { MatchConfig } from '../../core/types';
import { IGameStatsClient } from '../../telemetry/GameStatsHttpClient';

export class MemoryGame extends Game {
  constructor() {
    super(
      'memory-match',
      'Memory Match',
      '1.0.0',
      'Empareja cartas descubriendo sus símbolos. Emite eventos válidos a GameStats conforme juegas.'
    );
  }

  public createMatch(_config: MatchConfig, statsClient: IGameStatsClient): MatchInstance {
    return new MemoryMatch(this, statsClient);
  }
}

interface Card {
  id: number;
  symbol: string;
  revealed: boolean;
  matched: boolean;
}

export class MemoryMatch extends MatchInstance {
  private cards: Card[] = [];
  private selected: number[] = [];
  private matchesFound: number = 0;
  private readonly totalPairs = 6;
  private readonly playerId = 'player-1';
  private clickHandler!: (e: MouseEvent) => void;

  protected setupInputs(): void {
    const symbols = ['⭐', '⚡', '🔥', '💎', '🚀', '🔮'];
    const deck = [...symbols, ...symbols]
      .sort(() => Math.random() - 0.5)
      .map((symbol, id) => ({ id, symbol, revealed: false, matched: false }));
    this.cards = deck;

    this.clickHandler = (e: MouseEvent) => {
      const rect = this.canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      this.handleClick(x, y);
    };
    this.canvas.addEventListener('click', this.clickHandler);
  }

  protected onMatchStart(): void {
    // 1. MATCH_STARTED: player forbidden, fields map, mode, maxPlayers
    this.emitEvent('MATCH_STARTED', {
      map: 'memory-grid',
      mode: 'cards',
      maxPlayers: 1
    }, null);

    // 2. PLAYER_JOINED: player required, field nickname
    this.emitEvent('PLAYER_JOINED', {
      nickname: 'MemoryPlayer',
      team: 'Solo'
    }, this.playerId);
  }

  private handleClick(x: number, y: number): void {
    if (this.selected.length === 2) return;

    const cols = 4;
    const cardW = 75;
    const cardH = 70;
    const gap = 12;
    const offsetX = 20;
    const offsetY = 20;

    const col = Math.floor((x - offsetX) / (cardW + gap));
    const row = Math.floor((y - offsetY) / (cardH + gap));

    if (col < 0 || col >= cols || row < 0 || row >= 3) return;
    const index = row * cols + col;

    if (index >= 0 && index < this.cards.length && !this.cards[index].revealed && !this.cards[index].matched) {
      this.cards[index].revealed = true;
      this.selected.push(index);

      // Emite PLAYER_ACTION con MOVE (x, y obligatorios según config/event_types.json)
      this.emitEvent('PLAYER_ACTION', {
        x: col,
        y: row
      }, this.playerId, 'MOVE');

      if (this.selected.length === 2) {
        setTimeout(() => this.checkMatch(), 600);
      }
    }
  }

  private checkMatch(): void {
    const [c1, c2] = this.selected;
    if (this.cards[c1].symbol === this.cards[c2].symbol) {
      this.cards[c1].matched = true;
      this.cards[c2].matched = true;
      this.matchesFound++;

      // Emite PLAYER_ACTION con PICKUP_ITEM (item obligatorio según config/event_types.json)
      this.emitEvent('PLAYER_ACTION', {
        item: `pair-${this.cards[c1].symbol}`
      }, this.playerId, 'PICKUP_ITEM');

      if (this.matchesFound === this.totalPairs) {
        this.stop('SCORE_LIMIT');
      }
    } else {
      this.cards[c1].revealed = false;
      this.cards[c2].revealed = false;
    }
    this.selected = [];
  }

  protected update(): void {}

  protected render(ctx: CanvasRenderingContext2D): void {
    ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

    const cols = 4;
    const cardW = 75;
    const cardH = 70;
    const gap = 12;
    const offsetX = 20;
    const offsetY = 20;

    // Header info
    ctx.fillStyle = '#00f0ff';
    ctx.font = '11px monospace';
    ctx.fillText(`Partida: ${this.matchId} | Parejas: ${this.matchesFound}/${this.totalPairs}`, 20, 15);

    this.cards.forEach((card, i) => {
      const col = i % cols;
      const row = Math.floor(i / cols);
      const x = offsetX + col * (cardW + gap);
      const y = offsetY + row * (cardH + gap);

      ctx.fillStyle = card.matched ? '#10b981' : card.revealed ? '#3b82f6' : '#1e293b';
      ctx.fillRect(x, y, cardW, cardH);
      ctx.strokeStyle = card.matched ? '#34d399' : '#334155';
      ctx.lineWidth = 2;
      ctx.strokeRect(x, y, cardW, cardH);

      if (card.revealed || card.matched) {
        ctx.fillStyle = '#ffffff';
        ctx.font = '26px sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(card.symbol, x + cardW / 2, y + cardH / 2);
      }
    });

    if (this.matchesFound === this.totalPairs) {
      ctx.fillStyle = 'rgba(0, 0, 0, 0.75)';
      ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
      ctx.fillStyle = '#10b981';
      ctx.font = 'bold 20px monospace';
      ctx.textAlign = 'center';
      ctx.fillText('¡VICTORIA!', this.canvas.width / 2, this.canvas.height / 2);
    }
  }

  protected onMatchFinish(reason: string): void {
    // MATCH_FINISHED: player forbidden, reason in ["TIME_LIMIT", "SCORE_LIMIT", "ABORTED"]
    const validReason = (reason === 'SCORE_LIMIT' || reason === 'TIME_LIMIT') ? reason : 'ABORTED';
    this.emitEvent('MATCH_FINISHED', {
      reason: validReason
    }, null);
  }

  protected cleanup(): void {
    this.canvas.removeEventListener('click', this.clickHandler);
  }
}
