# ha-mcp

[![GitHub release](https://img.shields.io/github/v/release/zorak1103/ha-mcp)](https://github.com/zorak1103/ha-mcp/releases/latest)
[![License](https://img.shields.io/github/license/zorak1103/ha-mcp)](LICENSE)
[![CI](https://github.com/zorak1103/ha-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/zorak1103/ha-mcp/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/zorak1103/ha-mcp?style=flat)](https://goreportcard.com/report/github.com/zorak1103/ha-mcp)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zorak1103/ha-mcp)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/zorak1103/ha-mcp.svg)](https://pkg.go.dev/github.com/zorak1103/ha-mcp)
[![Docker Hub](https://img.shields.io/docker/v/zorak1103/ha-mcp?label=Docker%20Hub&logo=docker)](https://hub.docker.com/r/zorak1103/ha-mcp)
[![Docker Pulls](https://img.shields.io/docker/pulls/zorak1103/ha-mcp?logo=docker)](https://hub.docker.com/r/zorak1103/ha-mcp)
[![Release](https://github.com/zorak1103/ha-mcp/actions/workflows/release.yml/badge.svg)](https://github.com/zorak1103/ha-mcp/actions/workflows/release.yml)
[![Renovate](https://github.com/zorak1103/ha-mcp/actions/workflows/renovate.yml/badge.svg)](https://github.com/zorak1103/ha-mcp/actions/workflows/renovate.yml)

Un servidor del Protocolo de Contexto del Modelo (MCP) que proporciona a los asistentes de IA acceso a Home Assistant, permitiendo el control del hogar inteligente y la gestión de automatizaciones.

## Características

- **40 Herramientas Especializadas**: Consultas de entidades, CRUD de automatizaciones, gestión de helpers, scripts, escenas, dispositivos, áreas, etiquetas, plantas, zonas, personas, etiquetas, rastros, planos, actualizaciones, tareas pendientes, calendarios, cámaras, paneles, registro del sistema y más
- **Arquitectura Híbrida**: WebSocket para la mayoría de las operaciones, API REST para CRUD de automatizaciones/scripts/escenas
- **CRUD Completo**: Crear, leer, actualizar y eliminar automatizaciones/scripts/escenas/helpers
- **Acceso Profundo al Sistema**: Consultar registros, analizar dependencias, acceder al logbook, validar configuración
- **Salida Flexible**: Lenguaje natural (optimizado para LLM) y formato JSON
- **Control de Acceso**: Modo solo lectura, lista blanca/negra, control detallado a nivel de acción
- **Reconexión Automática**: Reconexión automática con retroceso exponencial
- **Confirmación Post-Mutación**: Sondeo de estado automático después de crear/actualizar/eliminar para confirmar cambios

## vs. Otros Servidores MCP para Home Assistant

Existen dos alternativas: la [integración oficial de HA MCP](https://www.home-assistant.io/integrations/mcp_server) (integrada, ~10 herramientas basadas en intenciones) y la comunidad [homeassistant-ai/ha-mcp](https://github.com/homeassistant-ai/ha-mcp) (Python/FastMCP, 95+ herramientas).

Elige ha-mcp si necesitas:
- Gestión completa del ciclo de vida de automatizaciones/scripts/escenas/helpers (crear, editar, eliminar)
- Análisis avanzado (dependencias, referencias cruzadas, cobertura de automatizaciones)
- Administración del sistema (consultas de registros, validación de configuración, logbook, historial)
- Gestión de medios (navegador, transmisiones de cámara), HACS y acceso a paneles
- Selección confiable de herramientas para LLM — 41 herramientas consolidadas reducen los errores de selección en comparación con 95+ alternativas detalladas

Elige la integración oficial si necesitas seguridad a nivel de entidad o ninguna infraestructura externa.

Consulta [docs/feature-comparison.md](docs/feature-comparison.md) para una matriz de características detallada de tres vías.

## Instalación

### Desde Binario

Descarga la última versión desde la página de [Releases](../../releases).

```bash
# Linux/macOS
tar -xzf ha-mcp_linux_amd64.tar.gz
chmod +x ha-mcp
sudo mv ha-mcp /usr/local/bin/

# Windows: extrae ha-mcp_windows_amd64.zip y añádelo al PATH
```

### Desde el Código Fuente

Requiere Go 1.26 o posterior.

```bash
git clone https://github.com/zorak1103/ha-mcp.git
cd ha-mcp
task install-hooks  # instala el gancho pre-commit de git (auto-corrige gofmt en cada commit)
go build -o ha-mcp ./cmd/ha-mcp
```

### Paquetes de Linux

Los paquetes RPM y DEB están disponibles en las versiones:

```bash
sudo dpkg -i ha-mcp_amd64.deb   # Debian/Ubuntu
sudo rpm -i ha-mcp_amd64.rpm    # RHEL/Fedora
```

### Docker

```bash
docker pull zorak1103/ha-mcp:latest
docker run -d --name ha-mcp -p 8080:8080 \
  -e HA_URL=http://homeassistant.local:8123 \
  zorak1103/ha-mcp:latest
```

Consulta [docs/configuration.md](docs/configuration.md) para opciones de Docker, HTTPS/WSS, soporte de proxy y todas las variables de entorno.

## Inicio Rápido

1. Obtén un token de acceso de larga duración desde la página de perfil de tu Home Assistant.

2. Inicia el servidor:

```bash
# Con banderas
ha-mcp --ha-url http://homeassistant.local:8123 --ha-token tu-token

# O inicializa los archivos de configuración primero
ha-mcp init   # crea config.yaml y .env
ha-mcp        # inicia con el archivo de configuración
```

3. Conecta tu cliente de IA. Ejemplo para Claude Desktop:

```json
{
  "mcpServers": {
    "homeassistant": {
      "type": "http",
      "url": "http://localhost:8080",
      "headers": { "Authorization": "Bearer tu-token-de-acceso-de-ha" }
    }
  }
}
```

Consulta [docs/configuration.md](docs/configuration.md) para configuraciones de Cline, opencode y otros clientes.

### Comandos Disponibles

| Comando         | Descripción                                      |
| --------------- | ------------------------------------------------ |
| `ha-mcp`        | Inicia el servidor MCP                             |
| `ha-mcp init`   | Crea config.yaml y .env en el directorio actual |
| `ha-mcp config` | Muestra la configuración efectiva (tokens enmascarados)  |
| `ha-mcp --help` | Muestra ayuda y banderas disponibles                    |

## Herramientas Disponibles

41 herramientas organizadas por dominio. Referencia completa en [docs/tools.md](docs/tools.md).

Siete temas de orientación también están disponibles como recursos MCP bajo URIs `skill://ha-mcp/<slug>` (selección de formato, patrones de automatización, resiliencia de plantillas, selección de helpers, seguridad de paneles, renombrado de entidades, flujo de depuración).

| Categoría          | Cantidad | Destacados                                                                  |
| ----------------- | ----- | --------------------------------------------------------------------------- |
| Entidad            | 5     | `query_entities` (historial/estadísticas/estado), `get_state`, `analyze_entity`      |
| Registro           | 10    | `get_registry`, `manage_area/label/floor/zone/person/tag/entity/device`     |
| Automatización        | 1     | `manage_automation` (CRUD, alternar, cobertura, JSON Patch + parche semántico)   |
| Helpers           | 2     | `manage_helper` (26 tipos), `helper_action`                                 |
| Scripts y Escenas  | 2     | `manage_script`, `manage_scene` (CRUD + ejecutar/activar + JSON Patch + parche semántico) |
| Análisis          | 4     | `analyze_entity`, `get_entity_dependencies`, `analyze_target`, `find_references` |
| Servicios          | 2     | `call_service`, `list_services`                                             |
| Historial/Logbook   | 2     | Modos de `query_entities`, `get_logbook` (entradas + correlación)               |
| Paneles/Media  | 4     | `manage_dashboard` (JSON Patch + parche semántico), `browse_media`, `manage_camera`, `sign_media_path` |
| Calendarios y Tareas Pendientes | 2     | `manage_calendar`, `manage_todo`                                            |
| Sistema/Admin      | 7     | `get_system_info`, `validate_config`, `manage_update`, `manage_blueprint`   |
| Registros              | 1     | `manage_system_log` (listar entradas WARN/ERROR, limpiar búfer circular)            |
| HACS              | 1     | `manage_hacs` (listar, descargar, instalar, repos personalizados)                       |
| Orientación          | 1     | `get_skill` (action=list para descubrir habilidades, action=read para obtener contenido)  |

## Control de Acceso

ha-mcp proporciona modo solo lectura, lista blanca y filtro de lista negra a nivel de herramienta y acción:

```yaml
# config.yaml — monitoreo solo lectura
server:
  read_only: true

# O bloquear operaciones específicas
server:
  tool_filter:
    blacklist:
      - "call_service"
      - "manage_*:delete"
```

Consulta [docs/access-control.md](docs/access-control.md) para patrones glob, filtrado por categoría (`*:write`) y escenarios de ejemplo.

## Arquitectura

```
Cliente de IA → HTTP/JSON-RPC → Servidor MCP ha-mcp
                                    │
               ┌────────────────────┴────────────────────┐
               │ WebSocket (principal)                      │ API REST
               │ - Consultas de estado, llamadas de servicio           │ - CRUD de automatizaciones
               │ - CRUD de helpers, acceso a registros           │ - CRUD de scripts/escenas
               └────────────────────┬────────────────────┘
                                    │
                             Home Assistant
```

Consulta [docs/architecture.md](docs/architecture.md) para la estructura del proyecto, comandos de compilación y configuración de pruebas de integración.

## Solución de Problemas

Consulta [docs/troubleshooting.md](docs/troubleshooting.md) para problemas de conexión WebSocket, modo de depuración y soluciones a errores comunes.

## Desarrollo

**Requisitos previos:** Go 1.26+, golangci-lint v2, Docker (opcional)

```bash
go build -o ha-mcp ./cmd/ha-mcp    # Compilar
go test ./...                       # Pruebas unitarias
golangci-lint run --timeout=5m ./...  # Lint
```

Consulta [docs/architecture.md](docs/architecture.md) para la configuración de pruebas de integración y [docs/integration-tests.md](docs/integration-tests.md) para la documentación completa de la suite de pruebas.

## Contribuir

1. Haz un fork del repositorio y crea una rama de funcionalidad
2. Realiza tus cambios con pruebas
3. Asegúrate de que CI pase:
   ```bash
   golangci-lint run ./...
   go test -race ./...
   ```
4. Abre un Pull Request

### Guías para Pull Requests

- Asegúrate de que las comprobaciones de CI pasen (lint, pruebas, escaneos de seguridad)
- Actualiza la documentación si es necesario
- Añade pruebas para la nueva funcionalidad
- Mantén los commits enfocados y atómicos

## Licencia

Licencia GPL-3.0 - consulta [LICENSE](LICENSE) para más detalles.

## Reconocimientos

- [Model Context Protocol](https://modelcontextprotocol.io/) especificación
- [Home Assistant WebSocket API](https://developers.home-assistant.io/docs/api/websocket)
- [coder/websocket](https://github.com/coder/websocket) - Biblioteca WebSocket pura en Go
