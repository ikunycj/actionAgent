import { useAppSelector } from "@/app/store";
import { useGetHealthQuery, useGetMeQuery } from "@/shared/api/coreHttpApi";

export function AppBootstrap() {
  const coreBaseUrl = useAppSelector((state) => state.connection.coreBaseUrl);
  const accessToken = useAppSelector((state) => state.auth.accessToken);
  const sessionState = useAppSelector((state) => state.auth.sessionState);
  const role = useAppSelector((state) => state.auth.role);
  const actor = useAppSelector((state) => state.auth.actor);

  const shouldFetchIdentity =
    Boolean(coreBaseUrl && accessToken) &&
    (sessionState === "checking" || role === "anonymous" || !actor);

  useGetHealthQuery(undefined, {
    skip: !coreBaseUrl
  });
  useGetMeQuery(undefined, {
    skip: !shouldFetchIdentity
  });

  return null;
}
