# Screenshots e Diagramas

Este diretório contém os diagramas de arquitetura e fluxo de dados para a integração do `zabbix-postfix`.

---

## 1. Funcionamento Geral (Visão Macro)

Diagrama de visão macro mostrando como o servidor Zabbix interage periodicamente com o agente no servidor de email.

![Funcionamento Geral](./how-it-works.png)

---

## 2. Fluxo de Integração Zabbix-Postfix

Diagrama detalhado que demonstra a interação entre o Zabbix Server/Agent, o script Bash auxiliar e os binários Go (`pygtail` e `pflogsumm`) para ler e acumular estatísticas do log.

![Fluxo de Integração](./postfix_zabbix_flow.jpg)

---

## 3. Fluxo de Entrega do Servidor Postfix

Fluxo de entrega interna de email no Postfix mapeado contra as respectivas métricas capturadas e enviadas para o Zabbix (Received, Delivered, Rejected, Deferred, Queue Depth).

![Fluxo de Entrega](./postfix_delivery_flow.jpg)
