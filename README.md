# Lybel Skills

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/lybel-app/skills?color=11C47E&label=release)](https://github.com/lybel-app/skills/releases/latest)
[![Claude Skills](https://img.shields.io/badge/Claude-Skills-11C47E)](https://docs.claude.com/en/docs/claude-code/skills)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-11C47E)](https://github.com/lybel-app/skills/pulls)

> Skills do Claude para o time da **Lybel**. Pergunte em português; o Claude vai direto no Confluence, Jira e afins.

---

## O que é

`lybel-skills` é um repositório público que reúne **Claude Skills** prontas para o time da Lybel. Uma *skill* é um pacote de instruções + arquivos de referência que o Claude carrega sob demanda quando você faz uma pergunta que bate com o escopo dela.

Hoje só temos uma skill, mas o repo foi desenhado para crescer: próximas candidatas incluem `lybel-jira`, `lybel-figma`, `lybel-analytics`, etc.

## Skills disponíveis

| Skill | Resumo | Docs |
|---|---|---|
| **`lybel-docs`** | Assistente da base de conhecimento Confluence. Busca, cria e atualiza páginas no espaço Lybel em linguagem natural. Usa um CLI Go local (`page digest`, `page apply`, `search`) que retorna sub-KB em vez do ADF inteiro — drasticamente mais barato em tokens que o MCP Atlassian puro. MCP fica como fallback. | [SKILL.md](./skills/lybel-docs/SKILL.md) |

---

## Como funciona — modelo bootstrap

A `lybel-docs` é uma skill **timeless**: o repo não guarda dados específicos da Lybel (nomes de advisors, lista de investidores, page IDs de cada parceiro). Em vez disso:

- **Ao usar a skill, o Claude sempre consulta a Home do Confluence primeiro** (pageId `164232`). A Home é a fonte de verdade — mantém taxonomia atual, aliases, status e o índice de page IDs.
- O repo só fornece **estrutura, workflows e templates** — instruções genéricas que não envelhecem.
- **Nenhum dado específico vive no repo** — por isso é safe deixar público no GitHub.
- Os arquivos em `reference/` são apenas **fallback** quando o Confluence está inacessível.

**Para customizar pra outra empresa**: troque `cloudId` e `pageId` da Home no frontmatter e no corpo de [`skills/lybel-docs/SKILL.md`](./skills/lybel-docs/SKILL.md). Crie a Home no seu próprio Confluence seguindo o mesmo padrão (taxonomia + aliases + index).

## Por que CLI em vez de só MCP

O servidor MCP da Atlassian é genérico e devolve o ADF inteiro de cada página (10–40 KB de JSON). Em uma sessão típica de research + edição de docs, isso facilmente queima a janela de contexto. O `lybel-docs` CLI vive no diretório da skill (`~/.claude/skills/lybel-docs/bin/`) e oferece comandos enxutos:

- **`home --refresh`** — uma vez por sessão, baixa a Home do Confluence e guarda em `~/.cache/lybel-docs/home.json` (digest + texto renderizado + ADF parseado). Daí em diante, todo `home --query "termo"` / `--show` / `--digest` é 100% local — zero chamadas pra API.
- **`page digest --page-id ID`** — devolve título, versão, outline de headings, macros e word count em ~500 bytes (vs 10–40 KB do `getConfluencePage`). Resolve a maioria das perguntas "o que tem nessa página?".
- **`page apply --page-id ID --replace-section "X" --fragment file.md`** — GET → edit section-level → PUT atômico, com retry automático em 409 (alguém editou no meio). Suporta também `--table-add-row` / `--table-remove-row` pra atualizar tabelas dentro de seções. O ADF nunca passa pelo contexto do LLM. Macros fora da seção alterada são preservadas byte-a-byte.
- **`search "termo"`** — busca CQL com saída TSV compacta (`pageId\ttitle\turl\texcerpt`).
- **`page get --format export_view`** — quando precisa do conteúdo, devolve o HTML renderizado (~2× menor que ADF).

**Invariante de segurança:** o cache é read-only. Toda escrita (`page apply`, `index add`, etc.) faz GET fresh do ADF antes do PUT — assim você nunca sobrescreve uma alteração feita em outra máquina.

O Claude usa o CLI quando ele existe e cai no MCP automaticamente quando não. Resultado: sessão de docs típica fica 10–50× mais barata em tokens.

---

## Quick start — caminho fácil (não precisa ser dev)

Se você não mexe com código, segue esses 5 passos:

1. **Instale o Claude Desktop** — baixe em [claude.ai/download](https://claude.ai/download) e faça login com sua conta Lybel.
2. **Baixe o instalador** para o seu sistema:
   - Windows: [install.bat](https://raw.githubusercontent.com/lybel-app/skills/main/install.bat)
   - macOS/Linux: [install.sh](https://raw.githubusercontent.com/lybel-app/skills/main/install.sh)
3. **Duplo-clique no arquivo baixado.**
   - Windows: pode aparecer um aviso do SmartScreen — clique em "Mais informações" → "Executar mesmo assim".
   - macOS/Linux: se não abrir com duplo-clique, abra o Terminal, vá na pasta de Downloads e rode `bash install.sh`.
4. **Reabra o Claude Desktop** e vá na aba **Code**. Em **Settings → Integrations**, conecte sua conta Atlassian (OAuth — só autorizar na janela que abre).
5. **Pronto.** Agora é só perguntar. Exemplos que funcionam de primeira:
   - *"onde cadastro um novo advogado?"*
   - *"me dá a página de parceiros"*
   - *"quais aceleradoras a Lybel está participando?"*
   - *"cria uma página de ata de reunião com o Itaú"*

> **Precisa atualizar?** Baixe o instalador de novo e duplo-clique. Ele sobrescreve a versão anterior — é seguro re-executar.

---

## Quick start — caminho dev

```bash
# 1. Clone
git clone https://github.com/lybel-app/skills.git
cd lybel-skills

# 2. Symlink da skill para o diretório do Claude
ln -s "$(pwd)/skills/lybel-docs" ~/.claude/skills/lybel-docs

# 3. (Recomendado) Build do CLI Go — habilita digest/apply/search e reduz custo de tokens
cd cli/lybel-docs && make install
# (Build padrão instala em ~/.claude/skills/lybel-docs/bin/lybel-docs)
# Configurar credenciais Atlassian:
lybel-docs setup
cd -

# 4. Reinicie o Claude Code
#    A skill aparece automaticamente quando você faz uma pergunta relevante.
```

No Windows, troque o `ln -s` por um **diretório junction** (`mklink /J`) ou copie a pasta.

---

## Uso típico

### 1. "Onde fica a página X?"

```
Você: onde fica a página de governança?

Claude: Achei no Confluence:
- Governança Lybel — estrutura de comitês, cadência de board meetings e RACI
  https://lybel.atlassian.net/wiki/spaces/lybel/pages/229891
```

### 2. Criar página nova com template

```
Você: cria uma página de parceiro novo: Banco Fictício

Claude: Vou criar em Parceiros → Bancos usando o template "Ficha de Parceiro".
        Título sugerido: "Banco Fictício — Ficha de Parceiro"
        Parent: Parceiros / Bancos (pageId 164500)
        Confirma?

Você: sim

Claude: Criado: https://lybel.atlassian.net/wiki/spaces/lybel/pages/998877
```

### 3. Listar páginas de uma categoria

```
Você: quais aceleradoras a gente tem mapeadas?

Claude: No espaço Lybel, categoria Aceleração:
- Cubo Itaú — 2026 cohort (Aceleração/Programas)
- Distrito Fintech — em avaliação
- Endeavor — contato preliminar
...
```

### 4. Atualizar página existente

```
Você: adiciona na página do Cubo Itaú que a reunião de kickoff ficou pra 05/05

Claude: Encontrei a página "Cubo Itaú — 2026 cohort" (version 7).
        Vou adicionar na seção "Cronograma" a linha:
        "05/05 — Kickoff com mentores"
        Confirma?
```

### 5. Status de algo

```
Você: qual o status da parceria com Nubank?

Claude: Página "Nubank — Parceria" (última atualização: 2026-04-12):
        Label: "status:negociacao"
        Responsável: Diego Clair
        Próximo passo: enviar proposta revisada até 25/04
```

---

## Desenvolvendo

### Estrutura do repo

```
lybel-skills/
├── skills/                    # Skills publicadas (SKILL.md + reference/)
│   └── lybel-docs/
│       ├── SKILL.md           # Frontmatter + instruções
│       ├── reference/         # Docs auxiliares (taxonomia, aliases, templates)
│       └── bin/               # Binário Go opcional (ADF builder)
├── cli/                       # Código-fonte dos binários Go
│   └── lybel-docs/
├── install.bat                # Instalador Windows
├── install.sh                 # Instalador macOS/Linux
├── LICENSE                    # MIT
└── README.md                  # Este arquivo
```

### Como adicionar uma skill nova

1. Crie `skills/<nome-da-skill>/SKILL.md` seguindo o formato de [skills.md](https://docs.claude.com/en/docs/claude-code/skills).
2. Adicione arquivos de referência em `skills/<nome>/reference/`.
3. Teste localmente via symlink (veja [Quick start — caminho dev](#quick-start--caminho-dev)).
4. Abra PR. Após merge, a release é publicada automaticamente pelo GitHub Actions (em breve).

### Convenções

- `name` no frontmatter: lowercase com hífens, máx 64 chars.
- `description`: máx 1024 chars, inclua triggers (frases que ativam a skill).
- Corpo do SKILL.md em **português (Brasil)** — frontmatter fica em inglês.
- Referências usam paths relativos: `reference/foo.md`, **não** URLs absolutos.

### Rodar testes (quando existirem)

```bash
make test          # testa o CLI Go
make lint          # golangci-lint
```

---

## FAQ

**Preciso ter conta GitHub?**
Não. O repo é público e o ZIP do instalador também. Só baixar e rodar.

**Preciso ter git instalado?**
Não, se você usar o instalador (caminho fácil). Só precisa de git se for desenvolvedor e quiser clonar.

**E se eu quiser atualizar a skill?**
Re-execute `install.bat` ou `install.sh`. O instalador é idempotente — ele apaga a versão anterior e instala a nova.

**Funciona em Linux?**
Sim. O `install.sh` detecta macOS e Linux automaticamente. Requer `curl` (ou `wget`) e `unzip`.

**O instalador precisa de permissão de administrador?**
Não. A skill é instalada no seu diretório de usuário (`~/.claude/skills/` ou `%USERPROFILE%\.claude\skills\`).

**Onde os arquivos ficam no meu computador?**
- Windows: `C:\Users\<seu-usuario>\.claude\skills\lybel-docs\`
- macOS/Linux: `~/.claude/skills/lybel-docs/`

**Como desinstalo?**
Apague a pasta acima. Pronto.

**A skill manda meus dados pra algum servidor?**
Não. A skill só roda dentro do seu Claude Desktop/Code e usa a integração Atlassian que você configurou. Nada de telemetria.

---

## License

[MIT](./LICENSE) © 2026 Lybel
