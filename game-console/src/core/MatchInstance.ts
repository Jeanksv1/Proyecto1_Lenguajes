import { Game } from './Game';
import { IGameStatsClient } from '../telemetry/GameStatsHttpClient';
import { EventEnvelopeFactory, IEventEnvelope } from '../telemetry/EventEnvelope';
import { MatchStatus } from './types';

export abstract class MatchInstance {
  protected readonly matchId: string;
  protected status: MatchStatus = MatchStatus.IDLE;
  protected canvas!: HTMLCanvasElement;
  protected ctx!: CanvasRenderingContext2D;
  private animationFrameId: number | null = null;
  private lastTimestamp: number = 0;

  constructor(
    public readonly game: Game,
    protected readonly statsClient: IGameStatsClient
  ) {
    // Encapsulación de ID único e irrepetible para la partida
    this.matchId = `m-${crypto.randomUUID().substring(0, 8)}`;
  }

  public getMatchId(): string {
    return this.matchId;
  }

  public getStatus(): MatchStatus {
    return this.status;
  }

  public init(container: HTMLElement, width: number = 380, height: number = 280): void {
    this.canvas = document.createElement('canvas');
    this.canvas.width = width;
    this.canvas.height = height;
    this.canvas.tabIndex = 0; // Habilitar foco para captura de teclado aislada
    this.canvas.style.outline = 'none';
    this.canvas.style.backgroundColor = '#0b0f19';
    this.canvas.style.borderRadius = '6px';
    this.canvas.setAttribute('data-match-id', this.matchId);

    const context = this.canvas.getContext('2d');
    if (!context) throw new Error('No se pudo inicializar CanvasRenderingContext2D');
    this.ctx = context;

    container.appendChild(this.canvas);
    this.setupInputs();
  }

  public start(): void {
    if (this.status === MatchStatus.RUNNING) return;
    this.status = MatchStatus.RUNNING;
    this.lastTimestamp = performance.now();
    this.onMatchStart();
    this.loop(this.lastTimestamp);
  }

  public pause(): void {
    if (this.status === MatchStatus.RUNNING) {
      this.status = MatchStatus.PAUSED;
      if (this.animationFrameId !== null) {
        cancelAnimationFrame(this.animationFrameId);
        this.animationFrameId = null;
      }
    }
  }

  public stop(reason: 'COMPLETED' | 'ABORTED' | 'TIME_LIMIT' | 'SCORE_LIMIT' = 'ABORTED'): void {
    if (this.status === MatchStatus.FINISHED) return;
    this.status = MatchStatus.FINISHED;
    if (this.animationFrameId !== null) {
      cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = null;
    }
    this.onMatchFinish(reason);
    this.cleanup();
  }

  // Emisión tipada y aislada de eventos hacia GameStats
  protected emitEvent<T extends Record<string, any> = Record<string, any>>(
    type: string,
    data: T,
    playerId: string | null = null,
    action?: string
  ): void {
    const envelope = EventEnvelopeFactory.create<T>(
      this.game.gameId,
      this.game.version,
      this.matchId,
      type,
      data,
      playerId,
      action
    );
    this.statsClient.sendEvent(envelope as unknown as IEventEnvelope);
  }

  private loop = (timestamp: number): void => {
    if (this.status !== MatchStatus.RUNNING) return;
    const deltaTime = (timestamp - this.lastTimestamp) / 1000;
    this.lastTimestamp = timestamp;

    this.update(deltaTime);
    this.render(this.ctx);

    this.animationFrameId = requestAnimationFrame(this.loop);
  };

  // Métodos de ciclo de vida polimórficos
  protected abstract setupInputs(): void;
  protected abstract onMatchStart(): void;
  protected abstract update(deltaTime: number): void;
  protected abstract render(ctx: CanvasRenderingContext2D): void;
  protected abstract onMatchFinish(reason: string): void;
  protected abstract cleanup(): void;
}
