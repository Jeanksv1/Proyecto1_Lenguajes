export enum MatchStatus {
  IDLE = 'IDLE',
  RUNNING = 'RUNNING',
  PAUSED = 'PAUSED',
  FINISHED = 'FINISHED'
}

export interface MatchConfig {
  container: HTMLElement;
  width?: number;
  height?: number;
}
