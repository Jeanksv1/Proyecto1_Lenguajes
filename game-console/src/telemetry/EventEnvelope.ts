export interface IEventEnvelope<T = Record<string, any>> {
  eventId: string;
  timestamp: string;      // RFC 3339 en UTC (ej: "2026-11-11T20:30:00.000Z")
  gameId: string;
  gameVersion: string;
  matchId: string;
  playerId: string | null;
  type: string;
  action?: string;
  data: T;
}

export class EventEnvelopeFactory {
  public static create<T = Record<string, any>>(
    gameId: string,
    gameVersion: string,
    matchId: string,
    type: string,
    data: T,
    playerId: string | null = null,
    action?: string
  ): IEventEnvelope<T> {
    return {
      eventId: crypto.randomUUID(),
      timestamp: new Date().toISOString(),
      gameId,
      gameVersion,
      matchId,
      playerId,
      type,
      ...(action ? { action } : {}),
      data: data || ({} as T)
    };
  }
}
