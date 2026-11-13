import { MatchInstance } from './MatchInstance';
import { IGameStatsClient } from '../telemetry/GameStatsHttpClient';
import { MatchConfig } from './types';

export abstract class Game {
  constructor(
    public readonly gameId: string,
    public readonly name: string,
    public readonly version: string,
    public readonly description: string
  ) {}

  public abstract createMatch(config: MatchConfig, statsClient: IGameStatsClient): MatchInstance;
}
