import { zodResolver } from "@hookform/resolvers/zod";
import { createFileRoute } from "@tanstack/react-router";
import type * as React from "react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { isApiError } from "@/lib/client";
import { useChangePassword, useMe, useUpdateProfile } from "@/queries";

export const Route = createFileRoute("/settings/account")({
	component: AccountSettingsPage,
});

const profileSchema = z.object({
	username: z.string().min(1, "Username is required"),
	display_name: z.string(),
	email: z.union([z.string().email("Invalid email"), z.literal("")]),
});
type ProfileValues = z.infer<typeof profileSchema>;

const passwordSchema = z
	.object({
		current_password: z.string().min(1, "Required"),
		new_password: z.string().min(8, "At least 8 characters"),
		confirm_password: z.string().min(1, "Required"),
	})
	.refine((v) => v.new_password === v.confirm_password, {
		message: "Passwords do not match",
		path: ["confirm_password"],
	});
type PasswordValues = z.infer<typeof passwordSchema>;

function Section({
	title,
	description,
	children,
}: {
	title: string;
	description: string;
	children: React.ReactNode;
}) {
	return (
		<section className="rounded-lg border border-border bg-card p-4 md:p-5">
			<h2 className="font-semibold text-foreground text-sm">{title}</h2>
			<p className="mt-0.5 mb-4 text-muted-foreground text-xs">{description}</p>
			<div className="space-y-3">{children}</div>
		</section>
	);
}

function Field({
	label,
	error,
	children,
}: {
	label: string;
	error?: string;
	children: React.ReactNode;
}) {
	return (
		<div className="space-y-1.5">
			<Label className="text-xs">{label}</Label>
			{children}
			{error ? <p className="text-danger text-xs">⚠ {error}</p> : null}
		</div>
	);
}

function AccountSettingsPage() {
	const meQuery = useMe();
	const updateMut = useUpdateProfile();
	const passwordMut = useChangePassword();

	const profileForm = useForm<ProfileValues>({
		resolver: zodResolver(profileSchema),
		defaultValues: { username: "", display_name: "", email: "" },
	});
	const passwordForm = useForm<PasswordValues>({
		resolver: zodResolver(passwordSchema),
		defaultValues: { current_password: "", new_password: "", confirm_password: "" },
	});

	const me = meQuery.data;
	useEffect(() => {
		if (me) {
			profileForm.reset({
				username: me.username,
				display_name: me.display_name,
				email: me.email,
			});
		}
	}, [me, profileForm]);

	const onProfile = (values: ProfileValues) => {
		updateMut.mutate(values, {
			onSuccess: () => toast.success("Profile saved"),
			onError: (e) => toast.error(isApiError(e) ? e.message : "Failed to save"),
		});
	};

	const onPassword = (values: PasswordValues) => {
		passwordMut.mutate(
			{ current_password: values.current_password, new_password: values.new_password },
			{
				onSuccess: () => {
					toast.success("Password changed");
					passwordForm.reset();
				},
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to change password"),
			},
		);
	};

	if (meQuery.isLoading) {
		return (
			<div className="space-y-3">
				<Skeleton className="h-48 w-full" />
				<Skeleton className="h-48 w-full" />
			</div>
		);
	}

	const pErr = profileForm.formState.errors;
	const wErr = passwordForm.formState.errors;

	return (
		<div className="space-y-4">
			<form onSubmit={profileForm.handleSubmit(onProfile)}>
				<Section title="Profile" description="Your account name and contact details.">
					<Field label="Username" error={pErr.username?.message}>
						<Input {...profileForm.register("username")} autoComplete="username" />
					</Field>
					<div className="grid gap-3 sm:grid-cols-2">
						<Field label="Display name" error={pErr.display_name?.message}>
							<Input {...profileForm.register("display_name")} />
						</Field>
						<Field label="Email" error={pErr.email?.message}>
							<Input {...profileForm.register("email")} type="email" autoComplete="email" />
						</Field>
					</div>
					<div className="flex justify-end pt-1">
						<Button type="submit" variant="success" loading={updateMut.isPending}>
							Save profile
						</Button>
					</div>
				</Section>
			</form>

			<form onSubmit={passwordForm.handleSubmit(onPassword)}>
				<Section title="Password" description="Change the password you use to sign in.">
					<Field label="Current password" error={wErr.current_password?.message}>
						<Input
							type="password"
							autoComplete="current-password"
							{...passwordForm.register("current_password")}
						/>
					</Field>
					<div className="grid gap-3 sm:grid-cols-2">
						<Field label="New password" error={wErr.new_password?.message}>
							<Input
								type="password"
								autoComplete="new-password"
								{...passwordForm.register("new_password")}
							/>
						</Field>
						<Field label="Confirm new password" error={wErr.confirm_password?.message}>
							<Input
								type="password"
								autoComplete="new-password"
								{...passwordForm.register("confirm_password")}
							/>
						</Field>
					</div>
					<div className="flex justify-end pt-1">
						<Button type="submit" variant="success" loading={passwordMut.isPending}>
							Change password
						</Button>
					</div>
				</Section>
			</form>
		</div>
	);
}
