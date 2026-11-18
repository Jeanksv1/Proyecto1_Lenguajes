import { IEventEnvelope } from './EventEnvelope';

export interface IGameStatsClient {
  sendEvent(envelope: IEventEnvelope<any>): Promise<boolean>;
  onEventSent(listener: (envelope: IEventEnvelope<any>, success: boolean) => void): void;
}

export class GameStatsHttpClient implements IGameStatsClient {
  private listeners: ((envelope: IEventEnvelope<any>, success: boolean) => void)[] = [];
  private matchQueues = new Map<string, Promise<void>>();

  constructor(private readonly baseUrl: string = '/api') {}

  public onEventSent(listener: (envelope: IEventEnvelope<any>, success: boolean) => void): void {
    this.listeners.push(listener);
  }


public sendEvent(envelope: IEventEnvelope<any>): Promise<boolean> {
  const matchId = envelope.matchId;

  const previous = this.matchQueues.get(matchId) ?? Promise.resolve();

  const current = previous
    .catch(() => {})
    .then(() => this.sendEventHttp(envelope));

  const queue = current.then(
    () => {},
    () => {}
  );

  this.matchQueues.set(matchId, queue);

  void queue.then(() => {
    if (this.matchQueues.get(matchId) === queue) {
      this.matchQueues.delete(matchId);
    }
  });

  return current;
}

private async sendEventHttp(envelope: IEventEnvelope<any>): Promise<boolean> {
  try {
    const url = `${this.baseUrl}/events`;

    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(envelope)
    });

    const ok = response.ok;

    if (!ok) {
      const errorText = await response.text();
      console.warn(
        `[GameStats] Error HTTP ${response.status} en ${envelope.type}:`,
        errorText
      );
    } else {
      console.info(
        `[GameStats] Evento emitido: ${envelope.type} (${envelope.matchId})`
      );
    }

    this.notifyListeners(envelope, ok);
    return ok;
  } catch (err) {
    console.error(
      `[GameStats] Error de red enviando ${envelope.type}:`,
      err
    );

    this.notifyListeners(envelope, false);
    return false;
  }
}


  private notifyListeners(envelope: IEventEnvelope<any>, success: boolean): void {
    this.listeners.forEach((listener) => {
      try {
        listener(envelope, success);
      } catch (e) {
        console.error('[GameStats] Error en listener de telemetría:', e);
      }
    });
  }
}
