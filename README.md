# RevisaTEC

Plataforma de revisión de código entre pares para proyectos grupales de cursos de programación del TEC.

## Descripción

RevisaTEC se conecta al repositorio de GitHub/GitLab de cada equipo y exige que todo cambio pase por revisión de un compañero antes de fusionarse a la rama principal, replicando el flujo de trabajo estándar de la industria. Calcula un índice de contribución individual objetivo y se lo muestra al profesor en un dashboard.

## Problema que resuelve

Hoy el profesor solo ve el resultado final de un repositorio, sin visibilidad de si hubo control de calidad entre compañeros ni de cuánto aportó cada estudiante. Esto genera notas grupales injustas y priva a los estudiantes de practicar el code review.

## Funcionalidades principales

- Configuración de rúbrica e indicaciones del curso con apoyo de IA (interpretación automática de PDF/DOCX/XLSX).
- Dashboard del profesor con métricas por grupo, próximas entregas e inconvenientes.
- Detalle de grupo con nota estimada, evidencia de PRs/commits y contribución por estudiante.
- Síntesis del avance y retroalimentación (propuesta de la IA + versión editable del profesor).
- Calendario de entregas con avisos automáticos.
- Detección de inconvenientes (commits de prueba, errores de entrega).
- Panel del estudiante con su propio aporte, retroalimentación publicada, edición de datos del grupo (integrantes, repositorio) y calendario filtrado a su propio grupo.
- Inicio de sesión con Google (OAuth2), con redirección automática a la vista correspondiente según el rol del usuario.

## Arquitectura

Microservicios comunicados por cola de mensajes: **Repo-Integration**, **Review-Workflow**, **Contribution-Metrics**, **Reports**, **Notifications**, **Rubric-AI** y **Calendar**. Backend en Azure App Service, expuesto vía Azure API Management. Autenticación OAuth2 (Google/GitHub). Ver el documento formal (`RevisaTEC_Documento_Formal.docx`) para el detalle completo.

## Stack tecnológico

| Capa | Tecnología |
|---|---|
| Frontend | React 19 + Vite |
| Backend | Azure App Service + Azure API Management |
| CI/CD | GitHub Actions (lint, build; versionado automático por git tag) |
| Autenticación | OAuth2 (Google, GitHub) |
| Base de datos | Por definir en Fase 2 |

## Instalación y uso

```bash
git clone https://github.com/Crisaraya21/revisatec.git
cd revisatec
npm install
npm run dev
```

Variables de entorno (`.env.local`):

```
VITE_API_BASE_URL=<url del gateway de Azure API Management>
```

## Equipo

- **Marlon Fabian Aragón Arce** — UX/UI: heurísticas, accesibilidad, wireframes, sistema de diseño, mockups de alta fidelidad e integración del login con Google.
- **Cristopher Daniel Araya Vega** — Infraestructura: repositorio, diagrama de arquitectura, Azure, Mock Services y pipeline CI/CD.
- **Leiner Josué Aguilar González (ZERO)** — Documentación formal, informe UX/UI y README.

## Capturas de pantalla

| | |
|---|---|
| **Login** (correo institucional o Google) | **Dashboard del profesor** |
| ![Login](./screenshots/01-login.png) | ![Dashboard](./screenshots/02-dashboard-profesor.png) |
| **Crear grupo** (modal) | **Configuración del curso** (rúbrica e IA) |
| ![Crear grupo](./screenshots/03-crear-grupo.png) | ![Configuración del curso](./screenshots/04-configuracion-curso.png) |
| **Calendario** | **Inconvenientes** |
| ![Calendario](./screenshots/05-calendario.png) | ![Inconvenientes](./screenshots/06-inconvenientes.png) |
| **Detalle de grupo** (contribución por estudiante) | **Retroalimentación** (IA + profesor) |
| ![Detalle de grupo](./screenshots/11-detalle-grupo.png) | ![Retroalimentación](./screenshots/07-retroalimentacion.png) |
| **Panel del estudiante** | **Mi grupo — modo oscuro** |
| ![Panel del estudiante](./screenshots/08-panel-estudiante.png) | ![Mi grupo modo oscuro](./screenshots/09-mi-grupo-modo-oscuro.png) |
| **Calendario — vista móvil (responsive)** | |
| ![Calendario móvil](./screenshots/10-calendario-movil.png) | |

> Capturas tomadas sobre el sistema desplegado. Cubren modo claro/oscuro y vista responsive (escritorio/móvil).

## Estructura del repositorio

```
revisatec/
├── docs/                  # Diagrama de arquitectura y endpoints de Mock Services
├── public/                # Assets estáticos
├── src/
│   ├── api/               # Cliente de la API (Mock Services)
│   ├── components/        # Componentes reutilizables (Icons, formularios, banners)
│   ├── context/            # AuthContext, ThemeContext
│   ├── hooks/              # usePaginatedList
│   └── pages/              # Login, Groups, GroupDetail, Rubric, Calendar, Feedback,
│                           #   Issues, StudentDashboard, StudentFeedback, StudentGroup
├── .github/workflows/      # CI (lint/build) y despliegue a Azure Static Web Apps
└── README.md
```

## Créditos

- [React](https://react.dev/) — biblioteca de UI
- [Vite](https://vitejs.dev/) — bundler y entorno de desarrollo
- [React Router](https://reactrouter.com/) — enrutamiento
- [Azure Static Web Apps](https://azure.microsoft.com/products/app-service/static/) — hosting
- [Azure API Management](https://azure.microsoft.com/products/api-management/) — gateway y Mock Services
- [GitHub Actions](https://github.com/features/actions) — CI/CD
- [Figma](https://www.figma.com/) — diseño UI/UX

## Licencia

Proyecto académico — Instituto Tecnológico de Costa Rica (TEC San Carlos), curso IC-5821.
