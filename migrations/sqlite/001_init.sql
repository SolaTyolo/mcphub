create table if not exists schema_migrations (
  version    text primary key,
  applied_at text not null
);

create table if not exists agents (
  id                   text primary key,
  name                 text not null unique,
  system_prompt        text not null default '',
  response_schema      text,
  response_description text,
  llm_base_url         text,
  llm_api_key          text,
  llm_model            text,
  vision_model         text,
  enabled              integer not null default 1,
  created_at           text not null
);

create table if not exists mcp_servers (
  id          text primary key,
  name        text not null unique,
  transport   text not null default 'stdio',
  command     text,
  url         text,
  args        text not null default '[]',
  env         text not null default '{}',
  enabled     integer not null default 1,
  created_at  text not null
);

create table if not exists agent_mcp_servers (
  agent_id  text not null references agents(id) on delete cascade,
  server_id text not null references mcp_servers(id) on delete cascade,
  primary key (agent_id, server_id)
);
