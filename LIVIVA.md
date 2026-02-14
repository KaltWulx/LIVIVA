# LIVIVA - Local Intelligent Virtual Intelligence & Versatile Assistant

## Visión General

**LIVIVA** es un sistema de inteligencia artificial de código abierto que se ejecuta localmente en infraestructuras Linux, pero aprovecha **LLMs externos** (GitHub Copilot, Anthropic Claude, OpenAI, etc.) como motor de razonamiento. Construido sobre el **Agent Development Kit (ADK)** de Google, LIVIVA integra procesamiento de lenguaje natural predictivo, control de infraestructura IoT, análisis de datos en tiempo real y orquestación multi-agente.

La filosofía de LIVIVA se centra en la **ejecución local con inteligencia externa**: el sistema, sus datos, configuraciones y dispositivos viven en tu hardware; la capacidad de razonamiento se obtiene de los mejores LLMs disponibles mediante APIs externas. Tus dispositivos responden únicamente a tus comandos y tu asistente opera desde tu infraestructura.

> **"Tu infraestructura, tu control. La inteligencia viene de donde sea mejor."**

---

## Referencia: Modelo JARVIS

LIVIVA toma como referencia técnica el modelo de computación avanzada JARVIS (Just A Rather Very Intelligent System), despojado de su narrativa cinematográfica y enfocado en sus capacidades técnicas replicables:

### Características de Arquitectura

**Procesamiento de Lenguaje Natural (NLP) Predictivo**
No solo interpreta comandos de voz, sino que utiliza análisis de contexto para anticipar necesidades. La "personalidad" del sistema es en realidad un algoritmo de ajuste de tono diseñado para minimizar la fricción en la interacción humano-máquina.

**Aprendizaje Profundo (Deep Learning)**
El sistema evoluciona mediante la observación del comportamiento del usuario, optimizando flujos de trabajo y personalizando respuestas según patrones históricos de decisión.

**Multitarea Asincrónica**
Capacidad para gestionar procesos críticos (como soporte vital o seguridad) de forma aislada mientras mantiene interacciones sociales o de investigación en primer plano.

**Interfaz de Confianza**
El diseño de respuesta está orientado a la eficiencia y la reducción del estrés del operador, utilizando una modulación de voz calmada y autoritaria.

### Funciones Técnicas y Operativas

| Función | Descripción Técnica |
|---------|---------------------|
| **Domótica de Red Centralizada** | Control total de infraestructuras físicas: climatización, seguridad biométrica, iluminación y gestión de energía mediante sensores IoT. |
| **Análisis de Big Data en Tiempo Real** | Capacidad para filtrar y procesar bases de datos masivas, extrayendo correlaciones estadísticas y visualizándolas mediante modelos tridimensionales. |
| **Diagnóstico de Telemetría Humana** | Monitorización constante de biometría (ritmo cardíaco, niveles hormonales, saturación) para evaluar el estado físico del operador. |

### Integración de Hardware

El sistema opera bajo un esquema de computación ubicua, distribuido en una nube privada con acceso a periféricos como brazos robóticos de precisión, sistemas de visualización avanzada y servidores de alta densidad.

---

## Principios Fundamentales

### 1. Ejecución Local, Inteligencia Externa
- El sistema se ejecuta íntegramente en hardware local (binario Go + servicios)
- La capacidad de razonamiento proviene de **LLMs externos** (GitHub Copilot, Anthropic, OpenAI, etc.) vía APIs
- Los datos de configuración, sesiones y dispositivos permanecen en tu infraestructura
- Fallback a LLM local (Ollama) para operaciones cuando no hay conectividad
- Operaciones críticas de IoT funcionan sin dependencia de LLM

### 2. Arquitectura Multi-Agente
- Sistema distribuido de agentes especializados
- Comunicación inter-agente nativa vía ADK (`transfer`, `sub_agents`)
- Cada agente opera en su dominio de experticia

### 3. Linux-Native
- Optimizado para distribuciones Linux (Arch, Ubuntu, NixOS)
- Integración con systemd, cgroups, namespaces
- Aprovechamiento de capacidades del kernel Linux

### 4. Eficiencia Computacional
- Implementado en **Go** (rendimiento cercano a C, productividad superior)
- Consumo mínimo de recursos en idle (< 100MB RAM)
- Escalabilidad horizontal mediante goroutines

---

## Stack Tecnológico

### Lenguaje Principal: Go

LIVIVA está construido 100% en Go, aprovechando la implementación oficial del ADK de Google para este lenguaje. Go proporciona el balance perfecto entre rendimiento, facilidad de desarrollo e integración con el runtime asíncrono del ADK.

El ADK de Google tiene implementación completa en Python, Java y Go. La naturaleza async del runtime ADK se mapea perfectamente a las goroutines de Go, haciendo que la integración sea natural y eficiente.

### Tecnologías Satélite

| Componente | Tecnología | Propósito |
|------------|------------|-----------|
| **Runtime** | Go 1.23+ | Core del sistema |
| **Agent Framework** | Google ADK Go | Orquestación multi-agente |
| **Comunicación** | gRPC + MQTT | Inter-agente e IoT |
| **Base de Datos** | SQLite + libSQL (Turso) | Vectorial y relacional |
| **Mensajería** | NATS | Event streaming |
| **Contenedores** | Podman | Sandbox de tools |
| **Sistema** | systemd | Servicios Linux |
| **Web UI** | Templ (Go) + HTMX | Interfaz web |
| **CLI** | Cobra (Go) | Interfaz terminal |

---

## Arquitectura del Sistema

### Diagrama General

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           LIVIVA CORE                                    │
│                      (Go + Google ADK)                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                     AGENTE COORDINADOR                           │   │
│  │                  (LlmAgent - Gateway)                            │   │
│  │  - Enrutamiento de intenciones                                   │   │
│  │  - Gestión de contexto global                                    │   │
│  │  - Priorización de tareas                                        │   │
│  └──────────────┬───────────────────────────────────────────────────┘   │
│                 │                                                        │
│     ┌───────────┼───────────┬──────────────┬──────────────┐             │
│     ▼           ▼           ▼              ▼              │             │
│  ┌──────┐   ┌──────┐   ┌──────────┐   ┌──────────┐      │             │
│  │ NLP  │   │ IoT  │   │ Analytics│   │ Learning │      │             │
│  │Agent │   │Agent │   │  Agent   │   │  Agent   │      │             │
│  └──┬───┘   └──┬───┘   └────┬─────┘   └────┬─────┘      │             │
│     │          │           │              │              │             │
│     ▼          ▼           ▼              ▼              ▼             │
│  ┌────────────────────────────────────────────────────────────────┐   │
│  │              LAYER DE SERVICIOS (Go Interfaces)                │   │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌───────────┐ │   │
│  │  │  Home       │ │   Vector    │ │  Biometric  │ │  External │ │   │
│  │  │  Assistant  │ │    DB       │ │    API      │ │    APIs   │ │   │
│  │  │  (MQTT)     │ │ (sqlite-vec)│ │ (Bluetooth) │ │  (HTTP)   │ │   │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └───────────┘ │   │
│  └────────────────────────────────────────────────────────────────┘   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
            ┌──────────┐    ┌──────────┐    ┌──────────┐
            │   IoT    │    │ Wearables│    │ External │
            │ Devices  │    │ (BLE)    │    │  LLMs    │
            │(ESP32,   │    │          │    │(Copilot, │
            │ Arduino) │    │          │    │ Claude)  │
            └──────────┘    └──────────┘    └──────────┘
```

### Componentes Principales

#### 1. Coordinador Central

El punto de entrada del sistema es un binario Go que inicializa el contexto, carga la configuración desde archivo YAML, captura señales del sistema operativo Linux para shutdown graceful, y orquesta los diferentes subsistemas: el coordinador ADK, el gateway HTTP/gRPC, y los servicios de background.

Características clave:
- Context cancellation para shutdown limpio
- Integración con systemd (Type=notify)
- Signal handling para SIGINT/SIGTERM
- Inicialización ordenada de dependencias

#### 2. Agente NLP Predictivo

Agente especializado en procesamiento de lenguaje natural que extiende LlmAgent del ADK con capacidades predictivas. Implementa análisis de contexto para anticipar necesidades del usuario antes de que las exprese explícitamente.

Funcionalidades:
- Interpretación de comandos de voz y texto
- Análisis contextual para predicción de intenciones
- Ajuste dinámico de tono según estado del usuario
- Priorización automática de procesos críticos
- Acceso a contexto: historial de decisiones, estado biométrico, entorno, agenda

#### 3. Agente IoT Controller

Gestor centralizado de todos los dispositivos del ecosistema. Implementa la interfaz de tools del ADK para permitir que otros agentes controlen dispositivos mediante comandos declarativos.

Tools expuestas:
- `control_device`: Envía comandos a dispositivos específicos
- `get_sensor_data`: Obtiene lecturas de sensores en tiempo real
- `discover_devices`: Escanea y registra nuevos dispositivos automáticamente

Abstracciones de comunicación:
- WiFi/MQTT para ESP32 y Raspberry Pi Pico W
- RF433/915MHz traducido vía gateway serial para Arduino
- Bluetooth LE para wearables y sensores de bajo consumo

#### 4. Sistema de Memoria

Implementación de base de datos vectorial usando SQLite con extensión libSQL (Turso) para almacenar embeddings de memoria a largo plazo. Permite búsqueda semántica de experiencias previas y patrones de usuario.

Estructura de datos:
- Tabla de memorias con contenido, embedding vectorial (F32_BLOB), metadata JSON, timestamp
- Índice vectorial para búsqueda por similitud de coseno
- Almacenamiento local sin dependencias de servicios cloud

Operaciones principales:
- StoreMemory: Almacena nueva experiencia con embedding
- SearchSimilar: Recupera memoras relevantes por similitud semántica

---

## Flujo de Datos

### 1. Inicialización del Sistema

El sistema se instala como servicio systemd en Linux, permitiendo inicio automático, logging integrado con journalctl, y gestión mediante systemctl. La instalación incluye creación de usuario dedicado sin shell, directorios de configuración en /etc/liviva, y datos en /var/lib/liviva.

### 2. Flujo de una Interacción

```
Usuario: "Prepara el laboratorio para trabajar"
    │
    ▼
┌─────────────────────────────────────┐
│  Input Processor (STT/Texto)        │
│  - Si voz: Whisper local (CPU/GPU)  │
│  - Si texto: Directo                │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  Coordinador ADK (Go)               │
│  - Parsea intención                 │
│  - Consulta contexto en Vector DB   │
│  - Determina agentes necesarios     │
└───────┬───────────┬─────────────────┘
        │           │
        ▼           ▼
┌──────────────┐ ┌──────────────┐
│ Agente IoT   │ │ Agente NLP   │
│ - Luces ON   │ │ - Confirma   │
│ - Temp 22°C  │ │   acciones   │
│ - PCs wake   │ │              │
└──────┬───────┘ └──────────────┘
       │
       ▼
┌─────────────────────────────────────┐
│  MQTT Gateway                       │
│  - Publica a tópicos específicos    │
│  - ESP32: "lab/lights/main"         │
│  - RP2040: "lab/climate/control"    │
└──────────────┬──────────────────────┘
               │
               ▼
        [Dispositivos IoT]
               │
               ▼
┌─────────────────────────────────────┐
│  Respuesta al Usuario               │
│  "Laboratorio listo. Temperatura    │
│   a 22°C, iluminación al 80%,       │
│   estaciones de trabajo activas."   │
└─────────────────────────────────────┘
```

---

## Estructura del Proyecto

```
liviva/
├── cmd/
│   └── liviva/
│       └── main.go                 # Entry point
├── internal/
│   ├── agents/
│   │   ├── coordinator.go          # Agente principal ADK
│   │   ├── nlp.go                  # Procesamiento lenguaje
│   │   ├── iot.go                  # Control dispositivos
│   │   ├── analytics.go            # Análisis de datos
│   │   └── learning.go             # Aprendizaje adaptativo
│   ├── gateway/
│   │   ├── http.go                 # API REST
│   │   └── mqtt.go                 # Broker MQTT
│   ├── llm/
│   │   ├── provider.go             # Interfaz de proveedores LLM
│   │   ├── litellm.go              # Proxy LiteLLM (Copilot, Claude, etc.)
│   │   └── fallback.go             # Fallback a Ollama local
│   ├── iot/
│   │   ├── mqtt_client.go          # Cliente MQTT
│   │   ├── device_registry.go      # Registro dispositivos
│   │   └── protocols/
│   │       ├── wifi.go             # ESP32/WiFi
│   │       ├── rf.go               # RF433/915 traductor
│   │       └── ble.go              # Bluetooth LE
│   ├── memory/
│   │   ├── vector_store.go         # DB vectorial
│   │   ├── session_store.go        # Sesiones ADK
│   │   └── learning_engine.go      # Patrones usuario
│   ├── config/
│   │   └── config.go               # Configuración YAML/JSON
│   └── tools/
│       ├── system.go               # Tools sistema Linux
│       └── browser.go              # Automatización web
├── pkg/
│   └── api/                        # API pública
├── web/
│   ├── templates/                  # Templ (Go templates)
│   └── static/                     # CSS/JS
├── configs/
│   ├── liviva.service              # systemd unit
│   └── mosquitto.conf              # Config MQTT
├── deployments/
│   ├── docker/
│   │   └── Dockerfile
│   └── nix/
│       └── default.nix             # Nix flake
├── go.mod
├── go.sum
└── README.md
```

---

## Deployment en Linux

### Binario Nativo (Único Estándar)

Compilación cruzada para Linux AMD64 con CGO habilitado para SQLite. El binario se instala en `/usr/local/bin/` con un usuario de sistema dedicado y sin privilegios elevados. La integración con **systemd** es obligatoria para garantizar la persistencia, el reinicio automático tras fallos y el endurecimiento (hardening) de seguridad nativo del kernel Linux.

Pasos principales:
1. Compilar con flags de optimización (`go build -ldflags="-s -w"`)
2. Instalar el binario, archivos de configuración en `/etc/liviva` y directorio de datos en `/var/lib/liviva`
3. Crear un usuario de sistema dedicado (`liviva`) con permisos mínimos sobre sus recursos
4. Configurar, habilitar e iniciar el servicio systemd (`liviva.service`)

---

## Roadmap de Desarrollo

### Fase 0: Esqueleto (Semana 1-2)
- [ ] Inicializar proyecto Go con `go mod init`
- [ ] Instalar ADK Go: `go get google.golang.org/adk`
- [ ] Configurar LiteLLM proxy con LLM externo (Copilot/Claude)
- [ ] Crear Root Agent (coordinador) con un sub-agent de prueba
- [ ] Verificar ciclo completo: input → coordinador → sub-agent → respuesta

### Fase 1: Fundamentos (Mes 1-2)
- [ ] Implementar coordinador con `transfer` a sub-agents
- [ ] Agente NLP básico con LLM externo
- [ ] Integrar MQTT para IoT
- [ ] Sistema de configuración YAML
- [ ] Logging estructurado (zerolog)
- [ ] Tests unitarios (testing + testify)
- [ ] ADK Evaluation con test cases básicos

### Fase 2: Agentes Core (Mes 2-3)
- [ ] Agente IoT con tools (`control_device`, `get_sensor_data`)
- [ ] Vector DB con sqlite-vec para memoria
- [ ] `ParallelAgent` para tareas concurrentes
- [ ] `SequentialAgent` para flujos ordenados
- [ ] Fallback automático LLM externo → Ollama local
- [ ] Web UI básica (Templ + HTMX)
- [ ] CLI con Cobra

### Fase 3: Inteligencia (Mes 3-4)
- [ ] Sistema de aprendizaje de patrones
- [ ] Agente predictivo (anticipación)
- [ ] Memory service a largo plazo
- [ ] Optimización de prompts por usuario
- [ ] Callbacks ADK para logging y monitoreo

### Fase 4: Integraciones (Mes 4-5)
- [ ] Home Assistant integration
- [ ] Soporte ESP32/Arduino/RP2040
- [ ] Wearables (Bluetooth LE)
- [ ] Browser automation (rod/Playwright)

### Fase 5: Hardening (Mes 5-6)
- [ ] Sandbox de tools (Podman)
- [ ] Encriptación de datos sensibles
- [ ] Autenticación
- [ ] Auditoría completa

---

## Referencias

- [Google ADK Documentation](https://google.github.io/adk-docs/)
- [ADK Go Reference](https://pkg.go.dev/google.golang.org/adk)
- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [NATS Go Client](https://github.com/nats-io/nats.go)
- [libSQL Go Client](https://github.com/tursodatabase/libsql-client-go)
- [systemd Go Bindings](https://github.com/coreos/go-systemd)

---

## Licencia

MIT License - Tu infraestructura, tu inteligencia, tu control.

---

**"Tu infraestructura, tu control. La inteligencia viene de donde sea mejor."**

*Built with ❤️ and Go on Linux.*
