import { GameStatsDashboard } from './GameStatsDashboard';
import { GameConsole } from '../core/GameConsole';
import { MatchStatus } from '../core/types';

export class ConsoleUI {
  private appContainer: HTMLElement;
  private catalogContainer!: HTMLElement;
  private matchesGrid!: HTMLElement;
  private activeCountBadge!: HTMLElement;
  private telemetryList!: HTMLElement;

  constructor(
    private readonly console: GameConsole,
    rootElementId: string = 'app'
  ) {
    const el = document.getElementById(rootElementId);
    if (!el) throw new Error(`Elemento #${rootElementId} no encontrado.`);
    this.appContainer = el;
    this.buildLayout();
    this.setupTelemetryListener();
    new GameStatsDashboard(document.getElementById('gamestats-dashboard') as HTMLElement);
  }

  private buildLayout(): void {
    this.appContainer.innerHTML = `
      <header class="console-header">
        <div>
          <h1 class="console-title">🎮 GameConsole</h1>
          <p style="color: var(--text-muted); font-size: 0.9rem;">Consola Web Multijuego POO & Ingestión de Telemetría</p>
        </div>
        <div class="console-metrics">
          <div class="badge">GameStats: <span style="color: var(--accent-green);">/api/events</span></div>
          <div class="badge badge-active" id="active-matches-count">Partidas Activas: 0</div>
        </div>
      </header>

      <section class="catalog-section">
        <h2 class="section-title">Biblioteca de Juegos Disponibles</h2>
        <div class="catalog-grid" id="catalog-grid"></div>
      </section>

      <section class="viewport-section">
        <h2 class="section-title">Partidas en Ejecución (Concurrencia Aislada)</h2>
        <div class="matches-grid" id="matches-grid">
          <p id="empty-state" style="color: var(--text-muted); font-style: italic;">
            No hay partidas en ejecución. Haz clic en "▶ Iniciar Partida" para instanciar un juego.
          </p>
        </div>
      </section>

      <section class="telemetry-feed">
        <h2 class="section-title" style="font-size: 0.95rem; margin-bottom: 8px;">📡 Monitor de Eventos en Tiempo Real (GameStats Ingestion)</h2>
        <ul class="telemetry-list" id="telemetry-list">
          <li style="color: var(--text-muted);">Esperando eventos de partidas...</li>
        </ul>
      </section>

      <div id="gamestats-dashboard"></div>
    `;

    this.catalogContainer = document.getElementById('catalog-grid') as HTMLElement;
    this.matchesGrid = document.getElementById('matches-grid') as HTMLElement;
    this.activeCountBadge = document.getElementById('active-matches-count') as HTMLElement;
    this.telemetryList = document.getElementById('telemetry-list') as HTMLElement;

    this.renderCatalog();
  }

  public renderCatalog(): void {
    this.catalogContainer.innerHTML = '';
    const games = this.console.getAvailableGames();

    games.forEach((game) => {
      const card = document.createElement('div');
      card.className = 'game-card';
      card.innerHTML = `
        <div>
          <div class="game-header">
            <h3 class="game-name">${game.name}</h3>
            <span class="game-version">v${game.version}</span>
          </div>
          <p class="game-description">${game.description}</p>
        </div>
        <button class="btn-launch" data-game-id="${game.gameId}">
          ▶ Iniciar Partida
        </button>
      `;

      const btn = card.querySelector('.btn-launch') as HTMLButtonElement;
      btn.addEventListener('click', () => this.handleLaunchMatch(game.gameId));

      this.catalogContainer.appendChild(card);
    });
  }

  private handleLaunchMatch(gameId: string): void {
    const emptyState = document.getElementById('empty-state');
    if (emptyState) emptyState.remove();

    const viewportCard = document.createElement('div');
    viewportCard.className = 'viewport-card';

    const header = document.createElement('div');
    header.className = 'viewport-header';

    const gameContainer = document.createElement('div');
    gameContainer.className = 'game-viewport';

    const footer = document.createElement('div');
    footer.className = 'viewport-footer';

    viewportCard.appendChild(header);
    viewportCard.appendChild(gameContainer);
    viewportCard.appendChild(footer);
    this.matchesGrid.appendChild(viewportCard);

    // Lanzar partida concurrente aislada
    const match = this.console.launchMatch(gameId, gameContainer);
    const matchId = match.getMatchId();

    header.innerHTML = `
      <div>
        <strong>${match.game.name}</strong> 
        <span class="match-id-badge">#${matchId}</span>
      </div>
      <div class="viewport-controls">
        <button class="btn-action btn-pause">Pausar</button>
        <button class="btn-action btn-danger btn-stop">Finalizar</button>
      </div>
    `;

    footer.innerHTML = `
      <span>Estado: <b class="status-indicator" style="color: var(--accent-green);">${match.getStatus()}</b></span>
      <span>Click en el canvas para activar controles</span>
    `;

    const pauseBtn = header.querySelector('.btn-pause') as HTMLButtonElement;
    const stopBtn = header.querySelector('.btn-stop') as HTMLButtonElement;
    const statusText = footer.querySelector('.status-indicator') as HTMLElement;

    // Sincronizar la interfaz con los cambios automáticos de la partida.
    const syncMatchStatus = (): void => {
      const status = match.getStatus();
      statusText.textContent = status;
      statusText.style.color = status === MatchStatus.FINISHED
        ? '#f87171'
        : status === MatchStatus.PAUSED
          ? '#fbbf24'
          : 'var(--accent-green)';

      pauseBtn.disabled = status === MatchStatus.FINISHED;
      pauseBtn.textContent = status === MatchStatus.PAUSED ? 'Reanudar' : 'Pausar';
      this.updateActiveCount();
    };

    pauseBtn.addEventListener('click', () => {
      if (match.getStatus() === MatchStatus.RUNNING) {
        match.pause();
      } else if (match.getStatus() === MatchStatus.PAUSED) {
        match.start();
      }
      syncMatchStatus();
    });

    // El juego puede finalizar por victoria, derrota o empate sin pulsar botones.
    const statusTimer = window.setInterval(() => {
      if (!viewportCard.isConnected) {
        window.clearInterval(statusTimer);
        return;
      }
      syncMatchStatus();
    }, 200);

    stopBtn.addEventListener('click', () => {
      window.clearInterval(statusTimer);
      this.console.terminateMatch(matchId);
      viewportCard.remove();
      this.updateActiveCount();

      if (this.matchesGrid.querySelectorAll('.viewport-card').length === 0) {
        this.matchesGrid.innerHTML = `
          <p id="empty-state" style="color: var(--text-muted); font-style: italic;">
            No hay partidas en ejecución. Haz clic en "▶ Iniciar Partida" para instanciar un juego.
          </p>
        `;
      }
    });

    this.updateActiveCount();
  }

  private updateActiveCount(): void {
    const count = this.console.getActiveMatches().filter(
      match => match.getStatus() === MatchStatus.RUNNING || match.getStatus() === MatchStatus.PAUSED
    ).length;
    this.activeCountBadge.textContent = `Partidas Activas: ${count}`;
  }

  private setupTelemetryListener(): void {
    this.console.statsClient.onEventSent((envelope, success) => {
      if (this.telemetryList.children.length === 1 && this.telemetryList.children[0].textContent?.includes('Esperando')) {
        this.telemetryList.innerHTML = '';
      }

      const li = document.createElement('li');
      li.className = `telemetry-item ${success ? '' : 'error'}`;
      const time = envelope.timestamp.split('T')[1].replace('Z', '');
      const actionText = envelope.action ? ` (${envelope.action})` : '';

      li.innerHTML = `
        <span style="color: var(--text-muted);">${time}</span>
        <span style="color: var(--accent-cyan); font-weight: bold;">[${envelope.matchId}]</span>
        <span style="color: ${success ? 'var(--accent-green)' : 'var(--accent-red)'}; font-weight: bold;">${envelope.type}${actionText}</span>
        <span style="color: var(--text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${JSON.stringify(envelope.data)}</span>
      `;

      this.telemetryList.insertBefore(li, this.telemetryList.firstChild);

      // Limitar a los 20 eventos más recientes
      if (this.telemetryList.children.length > 20) {
        this.telemetryList.removeChild(this.telemetryList.lastChild!);
      }
    });
  }
}

