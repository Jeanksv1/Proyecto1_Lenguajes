import { Game } from '../../core/Game';
import { MatchInstance } from '../../core/MatchInstance';
import { MatchConfig } from '../../core/types';
import { IGameStatsClient } from '../../telemetry/GameStatsHttpClient';

export class SnakeGame extends Game {
  constructor() {
    super(
      'snake-match',
      'Snake Match',
      '1.0.0',
      'Controla la serpiente para comer manzanas. Cada movimiento y alimento emite telemetría estricta.'
    );
  }

  public createMatch(_config: MatchConfig, statsClient: IGameStatsClient): MatchInstance {
    return new SnakeMatch(this, statsClient);
  }
}

interface Point {
  x: number;
  y: number;
}

export class SnakeMatch extends MatchInstance {
  private snake: Point[] = [{ x: 10, y: 10 }, { x: 9, y: 10 }];
  private food: Point = { x: 5, y: 5 };
  private dir: Point = { x: 1, y: 0 };
  private nextDir: Point = { x: 1, y: 0 };
  private moveTimer: number = 0;
  private readonly stepInterval: number = 0.15; // Velocidad de avance (segundos)
  private readonly gridWidth = 20;
  private readonly gridHeight = 14;
  private readonly playerId = 'player-1';
  private score: number = 0;
  private keyListener!: (e: KeyboardEvent) => void;
  private isFocused: boolean = false;

  protected setupInputs(): void {
    this.keyListener = (e: KeyboardEvent) => {
      // AISLAMIENTO: Solo procesa teclas si este canvas específico tiene el foco
      if (document.activeElement !== this.canvas) return;

      switch (e.key) {
        case 'ArrowUp':
        case 'w':
        case 'W':
          if (this.dir.y === 0) this.nextDir = { x: 0, y: -1 };
          e.preventDefault();
          break;
        case 'ArrowDown':
        case 's':
        case 'S':
          if (this.dir.y === 0) this.nextDir = { x: 0, y: 1 };
          e.preventDefault();
          break;
        case 'ArrowLeft':
        case 'a':
        case 'A':
          if (this.dir.x === 0) this.nextDir = { x: -1, y: 0 };
          e.preventDefault();
          break;
        case 'ArrowRight':
        case 'd':
        case 'D':
          if (this.dir.x === 0) this.nextDir = { x: 1, y: 0 };
          e.preventDefault();
          break;
      }
    };

    window.addEventListener('keydown', this.keyListener);

    this.canvas.addEventListener('focus', () => { this.isFocused = true; });
    this.canvas.addEventListener('blur', () => { this.isFocused = false; });
  }

  protected onMatchStart(): void {
    // 1. MATCH_STARTED: player forbidden, required fields: map, mode, maxPlayers
    this.emitEvent('MATCH_STARTED', {
      map: 'snake-arena',
      mode: 'arcade',
      maxPlayers: 1
    }, null);

    // 2. PLAYER_JOINED: player required, field: nickname
    this.emitEvent('PLAYER_JOINED', {
      nickname: 'SnakePilot',
      team: 'Green'
    }, this.playerId);
  }

  protected update(deltaTime: number): void {
    this.moveTimer += deltaTime;
    if (this.moveTimer < this.stepInterval) return;
    this.moveTimer = 0;

    this.dir = this.nextDir;
    const head: Point = {
      x: this.snake[0].x + this.dir.x,
      y: this.snake[0].y + this.dir.y
    };

    // Colisión con paredes
    if (head.x < 0 || head.x >= this.gridWidth || head.y < 0 || head.y >= this.gridHeight) {
      this.stop('ABORTED');
      return;
    }

    // Colisión consigo misma
    if (this.snake.some(segment => segment.x === head.x && segment.y === head.y)) {
      this.stop('ABORTED');
      return;
    }

    this.snake.unshift(head);

    // Emite evento MOVE con coordenadas x, y (obligatorios en config/event_types.json)
    this.emitEvent('PLAYER_ACTION', {
      x: head.x,
      y: head.y
    }, this.playerId, 'MOVE');

    // Comer comida
    if (head.x === this.food.x && head.y === this.food.y) {
      this.score += 10;
      this.spawnFood();

      // Emite evento PICKUP_ITEM con campo item (obligatorio en config/event_types.json)
      this.emitEvent('PLAYER_ACTION', {
        item: 'apple'
      }, this.playerId, 'PICKUP_ITEM');
    } else {
      this.snake.pop();
    }
  }

  private spawnFood(): void {
    let valid = false;
    while (!valid) {
      const rx = Math.floor(Math.random() * this.gridWidth);
      const ry = Math.floor(Math.random() * this.gridHeight);
      if (!this.snake.some(s => s.x === rx && s.y === ry)) {
        this.food = { x: rx, y: ry };
        valid = true;
      }
    }
  }

  protected render(ctx: CanvasRenderingContext2D): void {
    ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

    const cellW = this.canvas.width / this.gridWidth;
    const cellH = this.canvas.height / this.gridHeight;

    // Fondo y borde si tiene foco
    if (this.isFocused) {
      ctx.strokeStyle = '#00f0ff';
      ctx.lineWidth = 2;
      ctx.strokeRect(1, 1, this.canvas.width - 2, this.canvas.height - 2);
    }

    // Dibujar Comida
    ctx.fillStyle = '#ef4444';
    ctx.beginPath();
    ctx.arc(
      this.food.x * cellW + cellW / 2,
      this.food.y * cellH + cellH / 2,
      Math.min(cellW, cellH) / 2.2,
      0,
      Math.PI * 2
    );
    ctx.fill();

    // Dibujar Serpiente
    this.snake.forEach((seg, i) => {
      ctx.fillStyle = i === 0 ? '#10b981' : '#059669';
      ctx.fillRect(seg.x * cellW + 1, seg.y * cellH + 1, cellW - 2, cellH - 2);
    });

    // Score & Focus indicator
    ctx.fillStyle = this.isFocused ? '#00f0ff' : '#9ca3af';
    ctx.font = '11px monospace';
    ctx.fillText(
      `Puntos: ${this.score} | ${this.isFocused ? '● ACTIVO (WASD)' : '○ Click para controlar'}`,
      10,
      14
    );

    if (this.status === 'FINISHED') {
      ctx.fillStyle = 'rgba(0, 0, 0, 0.75)';
      ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
      ctx.fillStyle = '#ef4444';
      ctx.font = 'bold 18px monospace';
      ctx.textAlign = 'center';
      ctx.fillText('FIN DE LA PARTIDA', this.canvas.width / 2, this.canvas.height / 2);
    }
  }

  protected onMatchFinish(reason: string): void {
    const validReason = (reason === 'SCORE_LIMIT' || reason === 'TIME_LIMIT') ? reason : 'ABORTED';
    this.emitEvent('MATCH_FINISHED', {
      reason: validReason
    }, null);
  }

  protected cleanup(): void {
    window.removeEventListener('keydown', this.keyListener);
  }
}
