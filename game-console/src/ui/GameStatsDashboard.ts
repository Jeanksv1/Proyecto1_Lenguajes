type MatchSummary = {
  matchId: string;
  gameId: string;
  status: string;
  complete: boolean;
  eventCount: number;
  players: string[];
  startedAt?: string;
  finishedAt?: string;
};

type AnalysisResult = {
  matchId: string;
  gameId: string;
  complete: boolean;
  matchStats?: { durationSeconds?: number; totalKills?: number; mvpPlayerId?: string | null };
  playerStats?: Array<{
    playerId: string;
    kills?: number;
    deaths?: number;
    kdRatio?: number;
    shots?: number;
    hits?: number;
    accuracy?: number;
    totalDamage?: number;
  }>;
  patterns?: unknown;
  raceStats?: unknown;
};

export class GameStatsDashboard {
  private readonly root: HTMLElement;
  private matches: MatchSummary[] = [];
  private selectedMatchId: string | null = null;

  constructor(root: HTMLElement) {
    this.root = root;
    this.root.innerHTML = `
      <section style="margin-top:24px;padding:18px;border:1px solid #283653;border-radius:8px;background:#141c2e;color:#e5edf9">
        <div style="display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap">
          <h2 style="margin:0;font-size:1rem;text-transform:uppercase;letter-spacing:.06em;color:#9cb4d4">📊 Estadísticas de GameStats</h2>
          <button type="button" id="gs-refresh" style="padding:9px 16px;background:#00a9ca;border:0;border-radius:5px;color:#06121b;font-weight:700;cursor:pointer">Actualizar estadísticas</button>
        </div>
        <p id="gs-message" role="status" style="color:#9cb4d4;font-size:.85rem">Consultando GameStats...</p>
        <div id="gs-summary" style="display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px;margin:16px 0"></div>
        <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(300px,1fr));gap:16px">
          <div style="min-width:0">
            <h3 style="font-size:.95rem;color:#d7e5f7">Historial de partidas</h3>
            <div id="gs-history" style="max-height:360px;overflow:auto"></div>
          </div>
          <div style="min-width:0">
            <h3 style="font-size:.95rem;color:#d7e5f7">Resultados de Scala</h3>
            <div id="gs-details" style="color:#9cb4d4;font-size:.9rem">Selecciona una partida analizada.</div>
          </div>
        </div>
      </section>`;
    this.element<HTMLButtonElement>('gs-refresh').addEventListener('click', () => void this.refresh());
    void this.refresh();
  }

  private element<T extends HTMLElement = HTMLElement>(id: string): T {
    const element = this.root.querySelector<T>(`#${id}`);
    if (!element) throw new Error(`No se encontró el elemento ${id}`);
    return element;
  }

  private setMessage(message: string): void {
    this.element('gs-message').textContent = message;
  }

  private statCard(label: string, value: string | number): HTMLElement {
    const card = document.createElement('div');
    card.style.cssText = 'background:#0c1322;border:1px solid #2a3b57;border-radius:7px;padding:14px';
    const name = document.createElement('div');
    name.style.cssText = 'color:#9cb4d4;font-size:.8rem;margin-bottom:8px';
    name.textContent = label;
    const number = document.createElement('strong');
    number.style.cssText = 'color:#2ed5e6;font-size:1.45rem';
    number.textContent = String(value);
    card.append(name, number);
    return card;
  }

  public async refresh(): Promise<void> {
    const button = this.element<HTMLButtonElement>('gs-refresh');
    button.disabled = true;
    this.setMessage('Actualizando partidas desde Go...');
    try {
      const response = await fetch('/api/matches');
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const data: unknown = await response.json();
      if (!data || typeof data !== 'object' || !('matches' in data) ||
          !Array.isArray((data as { matches: unknown }).matches)) {
        throw new Error('Respuesta inesperada de /api/matches');
      }
      this.matches = (data as { matches: MatchSummary[] }).matches;
      this.renderSummary();
      this.renderHistory();
      this.setMessage(`${this.matches.length} partidas recuperadas desde GameStats (Go).`);
      if (this.selectedMatchId && this.matches.some(m => m.matchId === this.selectedMatchId)) {
        await this.showResults(this.selectedMatchId);
      }
    } catch (error) {
      this.setMessage(`No se pudieron consultar las partidas: ${error instanceof Error ? error.message : String(error)}`);
    } finally {
      button.disabled = false;
    }
  }

  private renderSummary(): void {
    const root = this.element('gs-summary');
    root.replaceChildren();
    const analyzed = this.matches.filter(m => m.status === 'analyzed').length;
    const events = this.matches.reduce((sum, m) => sum + (Number(m.eventCount) || 0), 0);
    root.append(
      this.statCard('Partidas registradas', this.matches.length),
      this.statCard('Partidas analizadas', analyzed),
      this.statCard('Eventos registrados', events),
      this.statCard('Memory / Snake / Gato', [
        'memory-match', 'snake-match', 'gato'
      ].map(id => this.matches.filter(m => m.gameId === id).length).join(' / '))
    );
  }

  private renderHistory(): void {
    const root = this.element('gs-history');
    root.replaceChildren();
    if (this.matches.length === 0) {
      root.textContent = 'Todavía no hay partidas registradas.';
      return;
    }
    const ordered = [...this.matches].sort((a, b) =>
      (b.startedAt ?? '').localeCompare(a.startedAt ?? ''));
    for (const match of ordered) {
      const row = document.createElement('button');
      row.type = 'button';
      row.style.cssText = 'display:block;width:100%;text-align:left;background:#0c1322;color:#e5edf9;border:1px solid #2a3b57;border-radius:6px;padding:12px;margin-bottom:8px;cursor:pointer';
      const heading = document.createElement('strong');
      heading.textContent = `${match.gameId} · ${match.matchId}`;
      const info = document.createElement('div');
      info.style.cssText = 'color:#9cb4d4;font-size:.78rem;margin-top:5px';
      const date = match.startedAt ? new Date(match.startedAt).toLocaleString('es-CR') : 'Sin fecha';
      info.textContent = `${match.status} · ${match.eventCount} eventos · ${date}`;
      row.append(heading, info);
      row.addEventListener('click', () => void this.showResults(match.matchId));
      root.appendChild(row);
    }
  }

  private async showResults(matchId: string): Promise<void> {
    this.selectedMatchId = matchId;
    const root = this.element('gs-details');
    root.replaceChildren();
    root.textContent = 'Consultando resultados calculados por Scala...';
    try {
      const response = await fetch(`/api/matches/${encodeURIComponent(matchId)}/results`);
      if (this.selectedMatchId !== matchId) return;
      if (response.status === 404) {
        root.textContent = 'Los resultados de Scala aún no están disponibles para esta partida.';
        return;
      }
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const result = await response.json() as AnalysisResult;
      if (this.selectedMatchId !== matchId) return;
      root.replaceChildren();
      const title = document.createElement('p');
      title.style.cssText = 'color:#2ed5e6;font-weight:bold';
      title.textContent = `${result.gameId} · ${result.matchId}`;
      root.appendChild(title);
      const stats = result.matchStats;
      const metrics = document.createElement('div');
      metrics.style.cssText = 'display:grid;grid-template-columns:repeat(auto-fit,minmax(125px,1fr));gap:8px';
      metrics.append(
        this.statCard('Duración', stats?.durationSeconds == null ? 'N/D' : `${stats.durationSeconds.toFixed(2)} s`),
        this.statCard('Eliminaciones', stats?.totalKills ?? 'N/D'),
        this.statCard('Participantes', result.playerStats?.length ?? 0)
      );
      root.appendChild(metrics);
      const playersTitle = document.createElement('h4');
      playersTitle.textContent = 'Estadísticas por jugador';
      root.appendChild(playersTitle);
      if (!result.playerStats?.length) {
        const empty = document.createElement('p');
        empty.textContent = 'Sin estadísticas individuales.';
        root.appendChild(empty);
      } else {
        for (const player of result.playerStats) {
          const item = document.createElement('div');
          item.style.cssText = 'padding:9px;margin:6px 0;background:#0c1322;border-radius:5px;border:1px solid #2a3b57';
          const name = document.createElement('strong');
          name.textContent = player.playerId;
          const info = document.createElement('div');
          info.style.cssText = 'font-size:.8rem;color:#9cb4d4;margin-top:4px';
          info.textContent = `Kills: ${player.kills ?? 0} · Muertes: ${player.deaths ?? 0} · Disparos: ${player.shots ?? 0} · Aciertos: ${player.hits ?? 0} · Precisión: ${player.accuracy ?? 0}`;
          item.append(name, info);
          root.appendChild(item);
        }
      }
    } catch (error) {
      if (this.selectedMatchId === matchId) {
        root.textContent = `Error consultando Scala: ${error instanceof Error ? error.message : String(error)}`;
      }
    }
  }
}
