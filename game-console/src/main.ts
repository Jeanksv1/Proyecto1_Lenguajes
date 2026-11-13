import { GameConsole } from './core/GameConsole';
import { GameStatsHttpClient } from './telemetry/GameStatsHttpClient';
import { ConsoleUI } from './ui/ConsoleUI';
import { MemoryGame } from './games/memory/MemoryGame';
import { SnakeGame } from './games/snake/SnakeGame';

/**
 * Bootstrap de la Consola Web GameConsole
 */
function bootstrap(): void {
  console.log('🎮 Inicializando GameConsole...');

  // 1. Inyección de dependencia del cliente HTTP de GameStats
  // En desarrollo con Vite usamos '/api' que proxifica hacia 'http://localhost:8080'
  const statsClient = new GameStatsHttpClient('/api');

  // 2. Instanciación del orquestador central (GameConsole)
  const gameConsole = new GameConsole(statsClient);

  // 3. Registro formal de los videojuegos POO
  const memoryGame = new MemoryGame();
  const snakeGame = new SnakeGame();

  gameConsole.registerGame(memoryGame);
  gameConsole.registerGame(snakeGame);

  console.log(`✅ Juegos registrados: [${memoryGame.gameId}, ${snakeGame.gameId}]`);

  // 4. Montaje de la UI reactiva y concurrente
  new ConsoleUI(gameConsole, 'app');
}

document.addEventListener('DOMContentLoaded', bootstrap);
