import { Game } from '../../core/Game';
import { MatchInstance } from '../../core/MatchInstance';
import { MatchConfig, MatchStatus } from '../../core/types';
import { IGameStatsClient } from '../../telemetry/GameStatsHttpClient';

type Cell = 'x' | 'o' | 'e';
type Outcome = 'x' | 'o' | 'draw' | null;

const LINES = [
  [0, 1, 2], [3, 4, 5], [6, 7, 8],
  [0, 3, 6], [1, 4, 7], [2, 5, 8],
  [0, 4, 8], [2, 4, 6]
];

export class GatoGame extends Game {
  constructor() {
    super('gato', 'Gato - Tic Tac Toe', '1.0.0',
      'Juega contra la computadora: sus movimientos los decide Prolog mediante Minimax.');
  }

  public createMatch(_config: MatchConfig, statsClient: IGameStatsClient): MatchInstance {
    return new GatoMatch(this, statsClient);
  }
}

export class GatoMatch extends MatchInstance {
  private board: Cell[] = Array<Cell>(9).fill('e');
  private readonly playerId = 'player-1';
  private readonly aiId = 'prolog-ai';
  private waitingForAI = false;
  private errorMessage = '';
  private outcome: Outcome = null;
  private requestController: AbortController | null = null;
  private clickHandler!: (event: MouseEvent) => void;

  private getGeometry() {
    const size = Math.min(this.canvas.width - 40, this.canvas.height - 65);
    return { left: (this.canvas.width - size) / 2, top: 35, size, cell: size / 3 };
  }

  protected setupInputs(): void {
    this.clickHandler = (event: MouseEvent) => {
      if (this.status !== MatchStatus.RUNNING || this.waitingForAI || this.outcome) return;
      const rect = this.canvas.getBoundingClientRect();
      const x = (event.clientX - rect.left) * this.canvas.width / rect.width;
      const y = (event.clientY - rect.top) * this.canvas.height / rect.height;
      const { left, top, size, cell } = this.getGeometry();
      if (x < left || x >= left + size || y < top || y >= top + size) return;
      const col = Math.floor((x - left) / cell);
      const row = Math.floor((y - top) / cell);
      const index = row * 3 + col;
      if (this.board[index] !== 'e') return;

      this.board[index] = 'x';
      this.errorMessage = '';
      this.emitEvent('PLAYER_ACTION', { x: col, y: row }, this.playerId, 'MOVE');
      if (!this.finishIfNeeded()) void this.playAI();
    };
    this.canvas.addEventListener('click', this.clickHandler);
  }

  protected onMatchStart(): void {
    // start() también se invoca al reanudar: no duplicar los eventos iniciales.
    if (this.board.some(cell => cell !== 'e') || this.waitingForAI) return;
    // La partida nueva solo emite estos eventos una vez.
    if (this.started) return;
    this.started = true;
    this.emitEvent('MATCH_STARTED', { map: 'gato-3x3', mode: 'pve', maxPlayers: 2 }, null);
    this.emitEvent('PLAYER_JOINED', { nickname: 'Jugador', team: 'X' }, this.playerId);
    this.emitEvent('PLAYER_JOINED', { nickname: 'Prolog Minimax', team: 'O' }, this.aiId);
  }
  private started = false;

  private checkOutcome(): Outcome {
    for (const [a, b, c] of LINES) {
      if (this.board[a] !== 'e' && this.board[a] === this.board[b] && this.board[a] === this.board[c]) {
        return this.board[a] as 'x' | 'o';
      }
    }
    return this.board.every(cell => cell !== 'e') ? 'draw' : null;
  }

  private finishIfNeeded(): boolean {
    const result = this.checkOutcome();
    if (!result) return false;
    this.outcome = result;
    this.stop('SCORE_LIMIT');
    this.render(this.ctx); // El bucle se detiene al finalizar: pintar resultado final.
    return true;
  }

  private async playAI(): Promise<void> {
    this.waitingForAI = true;
    const controller = new AbortController();
    this.requestController = controller;
    try {
      const response = await fetch('/prolog/move', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ board: this.board, player: 'o' }),
        signal: controller.signal
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const data: unknown = await response.json();
      if (this.status === MatchStatus.FINISHED || controller.signal.aborted) return;
      if (!data || typeof data !== 'object' || !('move' in data)) {
        throw new Error('Respuesta inválida de Prolog');
      }
      const move = (data as { move: unknown }).move;
      if (typeof move !== 'number' || !Number.isInteger(move) || move < 0 || move > 8 || this.board[move] !== 'e') {
        throw new Error('Prolog devolvió una casilla no válida');
      }
      this.board[move] = 'o';
      this.emitEvent('PLAYER_ACTION', { x: move % 3, y: Math.floor(move / 3) }, this.aiId, 'MOVE');
      this.finishIfNeeded();
    } catch (error) {
      if (!controller.signal.aborted) {
        this.errorMessage = `No se pudo consultar Prolog: ${error instanceof Error ? error.message : 'error desconocido'}`;
      }
    } finally {
      if (this.requestController === controller) this.requestController = null;
      this.waitingForAI = false;
      if (this.status !== MatchStatus.FINISHED) this.render(this.ctx);
    }
  }

  protected update(_deltaTime: number): void {}

  protected render(ctx: CanvasRenderingContext2D): void {
    ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
    const { left, top, size, cell } = this.getGeometry();
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.font = '13px sans-serif';
    ctx.fillStyle = '#e2e8f0';
    ctx.fillText(`Tú: X  |  Prolog: O  |  ${this.waitingForAI ? 'Pensando...' : 'Tu turno'}`, this.canvas.width / 2, 18);

    ctx.strokeStyle = '#64748b';
    ctx.lineWidth = 3;
    for (let i = 1; i < 3; i++) {
      ctx.beginPath(); ctx.moveTo(left + i * cell, top); ctx.lineTo(left + i * cell, top + size); ctx.stroke();
      ctx.beginPath(); ctx.moveTo(left, top + i * cell); ctx.lineTo(left + size, top + i * cell); ctx.stroke();
    }
    this.board.forEach((value, index) => {
      if (value === 'e') return;
      ctx.fillStyle = value === 'x' ? '#38bdf8' : '#34d399';
      ctx.font = `bold ${Math.floor(cell * 0.65)}px sans-serif`;
      ctx.fillText(value.toUpperCase(), left + (index % 3 + 0.5) * cell, top + (Math.floor(index / 3) + 0.5) * cell);
    });
    if (this.errorMessage) {
      ctx.fillStyle = '#fca5a5';
      ctx.font = '11px sans-serif';
      ctx.fillText('Error de conexión con Prolog. Finaliza esta partida.', this.canvas.width / 2, this.canvas.height - 10);
    }
    if (this.outcome) {
      ctx.fillStyle = 'rgba(11, 15, 25, 0.85)';
      ctx.fillRect(left, top + size / 2 - 28, size, 56);
      ctx.fillStyle = this.outcome === 'x' ? '#38bdf8' : this.outcome === 'o' ? '#f87171' : '#fbbf24';
      ctx.font = 'bold 20px sans-serif';
      ctx.fillText(this.outcome === 'x' ? '¡Ganaste!' : this.outcome === 'o' ? 'Ganó Prolog' : '¡Empate!', this.canvas.width / 2, top + size / 2);
    }
  }

  protected onMatchFinish(reason: string): void {
    const validReason = reason === 'SCORE_LIMIT' || reason === 'TIME_LIMIT' ? reason : 'ABORTED';
    this.emitEvent('MATCH_FINISHED', { reason: validReason }, null);
  }

  protected cleanup(): void {
    this.requestController?.abort();
    this.canvas.removeEventListener('click', this.clickHandler);
  }
}
