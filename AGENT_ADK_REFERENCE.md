# Guía de Referencia para Agentes: Uso de la Documentación ADK

## Propósito
Este archivo instruye a los agentes para que, ante cualquier instrucción relacionada con agentes, skills, herramientas, workflows u operaciones similares, consulten la documentación oficial de ADK (Agent Development Kit) de Google antes de responder o ejecutar acciones.

---

## Instrucciones para Agentes

1. **Reconocimiento de términos clave**
   - Si el prompt del usuario contiene palabras como: `skill`, `agent`, `tool`, `workflow`, `multi-agent`, `deploy`, `integration`, `runtime`, `evaluation`, etc., debes buscar la información relevante en la documentación de ADK.

2. **Referencia principal**
   - Documentación oficial: [https://google.github.io/adk-docs/](https://google.github.io/adk-docs/)

3. **Mapeo de conceptos y secciones**

   | Palabra clave    | Sección relevante en ADK Docs                                         |
   |------------------|-----------------------------------------------------------------------|
   | skill           | [Skills for Agents](https://google.github.io/adk-docs/skills/)         |
   | agent           | [Agents](https://google.github.io/adk-docs/agents/)                    |
   | workflow        | [Workflow agents](https://google.github.io/adk-docs/agents/workflow-agents/) |
   | tool            | [Tools and Integrations](https://google.github.io/adk-docs/integrations/) |
   | multi-agent     | [Multi-agent systems](https://google.github.io/adk-docs/agents/multi-agents/) |
   | deploy          | [Deployment](https://google.github.io/adk-docs/deploy/)                |
   | evaluation      | [Evaluation](https://google.github.io/adk-docs/evaluate/)              |
   | runtime         | [Agent Runtime](https://google.github.io/adk-docs/runtime/)            |

4. **Respuesta sugerida**
   - Cuando recibas una instrucción relacionada, responde con el enlace directo a la sección relevante y un breve resumen de los pasos o conceptos clave.

   **Ejemplo:**
   > "Para crear una nueva skill, consulta la sección [Skills for Agents](https://google.github.io/adk-docs/skills/) de la documentación de ADK. Las skills permiten extender las capacidades de los agentes. Sigue los ejemplos y guías en esa sección."

5. **Automatización de búsqueda**
   - Si tienes acceso web, utiliza el buscador interno de la documentación para encontrar ejemplos, tutoriales o referencias específicas.

---

## Nota para desarrolladores
Para cualquier acción, configuración o extensión de agentes, skills, herramientas, workflows, etc., consulta siempre la documentación oficial de ADK. Los agentes deben estar configurados para buscar y referenciar automáticamente la sección correspondiente.

---

**Última actualización:** Febrero 2026
