import { Game } from './Game';
import { MatchInstance } from './MatchInstance';
import { IGameStatsClient } from '../telemetry/GameStatsHttpClient';

export class GameConsole {
  private registeredGames: Map<string, Game> = new Map();
  private activeMatches: Map<string, MatchInstance> = new Map();

  constructor(public readonly statsClient: IGameStatsClient) {}

  public registerGame(game: Game): void {
    this.registeredGames.set(game.gameId, game);
  }

  public getAvailableGames(): Game[] {
    return Array.from(this.registeredGames.values());
  }

  public launchMatch(gameId: string, container: HTMLElement): MatchInstance {
    const game = this.registeredGames.get(gameId);
    if (!game) throw new Error(`El juego "${gameId}" no está registrado.`);

    // Creación de una instancia completamente aislada (concurrente)
    const match = game.createMatch({ container }, this.statsClient);
    match.init(container);
    match.start();

    this.activeMatches.set(match.getMatchId(), match);
    return match;
  }

  public terminateMatch(matchId: string): void {
    const match = this.activeMatches.get(matchId);
    if (match) {
      match.stop('ABORTED');
      this.activeMatches.delete(matchId);
    }
  }

  public getActiveMatches(): MatchInstance[] {
    return Array.from(this.activeMatches.values());
  }

  public getMatch(matchId: string): MatchInstance | undefined {
    return this.activeMatches.get(matchId);
  }
}
