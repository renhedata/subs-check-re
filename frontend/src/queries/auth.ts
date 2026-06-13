import { useMutation, useQuery } from "@tanstack/react-query";
import { client } from "../lib/client";
import type { auth } from "../lib/client.gen";
import { queryKeys } from "./queryKeys";

export function useMe() {
	return useQuery({
		queryKey: queryKeys.me(),
		queryFn: () => client.auth.Me(),
		retry: false,
		staleTime: Number.POSITIVE_INFINITY,
	});
}

export function useLogin() {
	return useMutation({
		mutationFn: (p: auth.LoginParams) => client.auth.Login(p),
	});
}

export function useRegister() {
	return useMutation({
		mutationFn: (p: auth.RegisterParams) => client.auth.Register(p),
	});
}
