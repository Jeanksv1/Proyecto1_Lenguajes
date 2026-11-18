

import { GameConsole } from './core/GameConsole';
import { GameStatsHttpClient } from './telemetry/GameStatsHttpClient';
import { ConsoleUI } from './ui/ConsoleUI';
import { MemoryGame } from './games/memory/MemoryGame';
import { SnakeGame } from './games/snake/SnakeGame';
import { GatoGame } from './games/gato/GatoGame';

function bootstrap(): void {
  console.log('🎮 Inicializando GameConsole...');

  const statsClient = new GameStatsHttpClient('/api');
  const gameConsole = new GameConsole(statsClient);

  const memoryGame = new MemoryGame();
  const snakeGame = new SnakeGame();
  const gatoGame = new GatoGame();

  gameConsole.registerGame(memoryGame);
  gameConsole.registerGame(snakeGame);
  gameConsole.registerGame(gatoGame);

  console.log('✅ Juegos registrados: memory-match, snake-match, gato');

  new ConsoleUI(gameConsole, 'app');
}

document.addEventListener('DOMContentLoaded', bootstrap);

