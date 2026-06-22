import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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

export function useUpdateProfile() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (p: auth.UpdateProfileParams) => client.auth.UpdateProfile(p),
		onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.me() }),
	});
}

export function useChangePassword() {
	return useMutation({
		mutationFn: (p: auth.ChangePasswordParams) => client.auth.ChangePassword(p),
	});
}
