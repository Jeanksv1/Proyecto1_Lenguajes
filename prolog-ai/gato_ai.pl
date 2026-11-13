:- module(gato_ai, [
    movimiento_ia/3,
    iniciar_servidor/1,
    cli_move/2
]).

% ==============================================================================
% Dependencias para Servidor HTTP y JSON (SWI-Prolog)
% ==============================================================================
:- use_module(library(http/thread_httpd)).
:- use_module(library(http/http_dispatch)).
:- use_module(library(http/http_json)).
:- use_module(library(http/http_cors)).

:- set_setting(http:cors, [*]).

% ==============================================================================
% 1. BASE DE CONOCIMIENTO
% ==============================================================================
linea_ganadora([0, 1, 2]).
linea_ganadora([3, 4, 5]).
linea_ganadora([6, 7, 8]).
linea_ganadora([0, 3, 6]).
linea_ganadora([1, 4, 7]).
linea_ganadora([2, 5, 8]).
linea_ganadora([0, 4, 8]).
linea_ganadora([2, 4, 6]).

rival(x, o).
rival(o, x).

ganador(Tablero, Jugador) :-
    linea_ganadora([A, B, C]),
    nth0(A, Tablero, Jugador),
    nth0(B, Tablero, Jugador),
    nth0(C, Tablero, Jugador),
    Jugador \== e.

tablero_lleno(Tablero) :-
    \+ member(e, Tablero).

fin_juego(Tablero) :- ganador(Tablero, _), !.
fin_juego(Tablero) :- tablero_lleno(Tablero), !.

movimiento_valido(Tablero, Indice) :-
    nth0(Indice, Tablero, e).

reemplazar_nth0(0, [_|T], X, [X|T]) :- !.
reemplazar_nth0(I, [H|T], X, [H|R]) :-
    I > 0,
    I1 is I - 1,
    reemplazar_nth0(I1, T, X, R).

% ==============================================================================
% 2. MINIMAX CON PROFUNDIDAD
% ==============================================================================
tablero_vacio(Tablero) :-
    forall(member(C, Tablero), C == e).

% Apertura óptima en O(1)
movimiento_ia(Tablero, _JugadorIA, 4) :-
    tablero_vacio(Tablero), !.

movimiento_ia(Tablero, JugadorIA, MejorMovimiento) :-
    minimax(Tablero, JugadorIA, JugadorIA, 0, MejorMovimiento, _Punt).

minimax(Tablero, _JugadorActual, JugadorIA, Profundidad, none, Puntuacion) :-
    fin_juego(Tablero), !,
    evaluar_terminal(Tablero, JugadorIA, Profundidad, Puntuacion).

minimax(Tablero, JugadorActual, JugadorIA, Profundidad, MejorMov, MejorPunt) :-
    findall(M, movimiento_valido(Tablero, M), Movimientos),
    NuevaProf is Profundidad + 1,
    rival(JugadorActual, SigJugador),
    evaluar_movimientos(Movimientos, Tablero, JugadorActual, SigJugador, JugadorIA, NuevaProf, MovsPuntuados),
    ( JugadorActual == JugadorIA ->
        seleccionar_max(MovsPuntuados, MejorMov, MejorPunt)
    ;
        seleccionar_min(MovsPuntuados, MejorMov, MejorPunt)
    ).

evaluar_movimientos([], _, _, _, _, _, []).
evaluar_movimientos([M|RestoM], Tablero, JugadorActual, SigJugador, JugadorIA, Prof, [M-Punt | RestoEvaluados]) :-
    reemplazar_nth0(M, Tablero, JugadorActual, NuevoTablero),
    minimax(NuevoTablero, SigJugador, JugadorIA, Prof, _, Punt),
    evaluar_movimientos(RestoM, Tablero, JugadorActual, SigJugador, JugadorIA, Prof, RestoEvaluados).

evaluar_terminal(Tablero, JugadorIA, Profundidad, Puntuacion) :-
    ganador(Tablero, Ganador), !,
    ( Ganador == JugadorIA ->
        Puntuacion is 10 - Profundidad
    ;
        Puntuacion is Profundidad - 10
    ).
evaluar_terminal(Tablero, _, _, 0) :-
    tablero_lleno(Tablero), !.

seleccionar_max([M-P], M, P) :- !.
seleccionar_max([M1-P1 | Resto], MejorM, MejorP) :-
    seleccionar_max(Resto, M2, P2),
    ( P1 >= P2 -> (MejorM = M1, MejorP = P1) ; (MejorM = M2, MejorP = P2) ).

seleccionar_min([M-P], M, P) :- !.
seleccionar_min([M1-P1 | Resto], MejorM, MejorP) :-
    seleccionar_min(Resto, M2, P2),
    ( P1 =< P2 -> (MejorM = M1, MejorP = P1) ; (MejorM = M2, MejorP = P2) ).

% ==============================================================================
% 3. NORMALIZACIÓN
% ==============================================================================
normalizar_celda(Val, x) :- (Val == "x" ; Val == x ; Val == 'X'), !.
normalizar_celda(Val, o) :- (Val == "o" ; Val == o ; Val == 'O'), !.
normalizar_celda(_, e).

normalizar_jugador(Val, x) :- (Val == "x" ; Val == x ; Val == 'X'), !.
normalizar_jugador(_, o).

normalizar_tablero([], []).
normalizar_tablero([H|T], [NH|NT]) :-
    normalizar_celda(H, NH),
    normalizar_tablero(T, NT).

% ==============================================================================
% 4. HTTP HANDLERS
% ==============================================================================
:- http_handler('/health', handle_health, [method(get)]).
:- http_handler('/move', handle_move_cors, [method(options)]).
:- http_handler('/move', handle_move, [method(post)]).

handle_health(_Request) :-
    cors_enable,
    reply_json_dict(_{status: "ok", service: "prolog-gato-ai"}).

handle_move_cors(Request) :-
    cors_enable(Request, [methods([post, options])]),
    format('~n').

handle_move(Request) :-
    cors_enable(Request, [methods([post, options])]),
    http_read_json_dict(Request, DictIn),
    RawBoard = DictIn.board,
    RawPlayer = DictIn.player,
    normalizar_tablero(RawBoard, Tablero),
    normalizar_jugador(RawPlayer, JugadorIA),
    ( fin_juego(Tablero) ->
        reply_json_dict(_{error: "Partida ya finalizada", status: "finished"}, [status(400)])
    ;
        movimiento_ia(Tablero, JugadorIA, MejorMovimiento),
        reemplazar_nth0(MejorMovimiento, Tablero, JugadorIA, NuevoTablero),
        ( ganador(NuevoTablero, JugadorIA) -> Estado = "win"
        ; tablero_lleno(NuevoTablero)       -> Estado = "draw"
        ;                                      Estado = "ongoing"
        ),
        reply_json_dict(_{
            status: "success",
            move: MejorMovimiento,
            player: JugadorIA,
            gameState: Estado,
            board: NuevoTablero
        })
    ).

iniciar_servidor(Puerto) :-
    http_server(http_dispatch, [port(Puerto)]),
    format('~nServidor Prolog escuchando en el puerto ~w~n', [Puerto]).

cli_move(RawBoardList, RawPlayer) :-
    normalizar_tablero(RawBoardList, Tablero),
    normalizar_jugador(RawPlayer, JugadorIA),
    movimiento_ia(Tablero, JugadorIA, Mov),
    format('{"move": ~w, "player": "~w"}~n', [Mov, JugadorIA]).
