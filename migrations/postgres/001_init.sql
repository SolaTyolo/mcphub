create table if not exists agents (
  id                   uuid primary key,
  name                 text not null unique,
  system_prompt        text not null default '',
  response_schema      jsonb,
  response_description text,
  llm_base_url         text,
  llm_api_key          text,
  llm_model            text,
  vision_model         text,
  enabled              boolean not null default true,
  created_at           timestamptz not null
);

create table if not exists mcp_servers (
  id          uuid primary key,
  name        text not null unique,
  transport   text not null default 'stdio',
  command     text,
  url         text,
  args        jsonb not null default '[]',
  env         jsonb not null default '{}',
  enabled     boolean not null default true,
  created_at  timestamptz not null
);

create table if not exists agent_mcp_servers (
  agent_id  uuid not null references agents(id) on delete cascade,
  server_id uuid not null references mcp_servers(id) on delete cascade,
  primary key (agent_id, server_id)
);
