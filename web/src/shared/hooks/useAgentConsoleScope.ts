import { useMemo } from "react";
import { useLocation, useParams } from "react-router-dom";

import { useGetRuntimeAgentsQuery } from "@/shared/api/coreHttpApi";

function normalizeAgentId(value: string | null | undefined) {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
}

export function useAgentConsoleScope() {
  const params = useParams<{ agentId?: string }>();
  const location = useLocation();
  const query = useGetRuntimeAgentsQuery();

  const searchParams = new URLSearchParams(location.search);
  const routeAgentId = normalizeAgentId(params.agentId);
  const queryAgentId = normalizeAgentId(searchParams.get("agent"));
  const requestedAgentId = routeAgentId ?? queryAgentId;
  const availableAgents = query.data?.agents ?? [];
  const defaultAgentId =
    normalizeAgentId(query.data?.default_agent) ?? availableAgents[0]?.agent_id ?? null;
  const knownAgentIds = useMemo(
    () => new Set(availableAgents.map((agent) => agent.agent_id)),
    [availableAgents],
  );
  const activeAgentId =
    requestedAgentId && knownAgentIds.has(requestedAgentId) ? requestedAgentId : defaultAgentId;

  function buildScopedPath(view: string) {
    if (!activeAgentId) {
      return "/app";
    }
    const suffix = view.startsWith("/") ? view : `/${view}`;
    return `/app/agents/${activeAgentId}${suffix}`;
  }

  return {
    ...query,
    routeAgentId,
    requestedAgentId,
    defaultAgentId,
    availableAgents,
    activeAgentId,
    hasResolvedAgent: Boolean(activeAgentId),
    isFallback: Boolean(requestedAgentId && requestedAgentId !== activeAgentId),
    buildScopedPath
  };
}
