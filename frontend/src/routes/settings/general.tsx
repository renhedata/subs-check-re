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
import { useSettings, useUpdateSettings } from "@/queries";

export const Route = createFileRoute("/settings/general")({
	component: GeneralSettingsPage,
});

const httpUrl = z
	.string()
	.url("Must be a valid URL")
	.refine((u) => u.startsWith("http"), "Must be http(s)");

const formSchema = z.object({
	latency_test_url: httpUrl,
	speed_test_url: httpUrl,
	upload_test_url: z.union([httpUrl, z.literal("")]),
	smtp_host: z.string(),
	smtp_port: z.number().int().min(0).max(65535),
	smtp_user: z.string(),
	smtp_pass: z.string(),
	from: z.string(),
});

type FormValues = z.infer<typeof formSchema>;

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

function GeneralSettingsPage() {
	const settingsQuery = useSettings();
	const updateMut = useUpdateSettings();

	const form = useForm<FormValues>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			latency_test_url: "",
			speed_test_url: "",
			upload_test_url: "",
			smtp_host: "",
			smtp_port: 587,
			smtp_user: "",
			smtp_pass: "",
			from: "",
		},
	});

	const loaded = settingsQuery.data;
	useEffect(() => {
		if (loaded) {
			form.reset({
				latency_test_url: loaded.latency_test_url,
				speed_test_url: loaded.speed_test_url,
				upload_test_url: loaded.upload_test_url,
				smtp_host: loaded.email_config.smtp_host,
				smtp_port: loaded.email_config.smtp_port,
				smtp_user: loaded.email_config.smtp_user,
				smtp_pass: loaded.email_config.smtp_pass,
				from: loaded.email_config.from,
			});
		}
	}, [loaded, form]);

	const onSubmit = (values: FormValues) => {
		if (!loaded) return;
		updateMut.mutate(
			{
				...loaded,
				latency_test_url: values.latency_test_url,
				speed_test_url: values.speed_test_url,
				upload_test_url: values.upload_test_url,
				email_config: {
					smtp_host: values.smtp_host,
					smtp_port: values.smtp_port,
					smtp_user: values.smtp_user,
					smtp_pass: values.smtp_pass,
					from: values.from,
				},
			},
			{
				onSuccess: () => toast.success("Settings saved"),
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to save"),
			},
		);
	};

	if (settingsQuery.isLoading) {
		return (
			<div className="space-y-3">
				<Skeleton className="h-40 w-full" />
				<Skeleton className="h-56 w-full" />
			</div>
		);
	}

	const errors = form.formState.errors;

	return (
		<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
			<Section
				title="Connectivity tests"
				description="Endpoints used to measure node latency and bandwidth during checks."
			>
				<Field
					label="Latency test URL"
					error={errors.latency_test_url?.message}
				>
					<Input
						{...form.register("latency_test_url")}
						className="font-mono"
						placeholder="https://www.gstatic.com/generate_204"
					/>
				</Field>
				<div className="grid gap-3 sm:grid-cols-2">
					<Field
						label="Download test URL"
						error={errors.speed_test_url?.message}
					>
						<Input {...form.register("speed_test_url")} className="font-mono" />
					</Field>
					<Field
						label="Upload test URL (optional)"
						error={errors.upload_test_url?.message}
					>
						<Input
							{...form.register("upload_test_url")}
							className="font-mono"
						/>
					</Field>
				</div>
			</Section>

			<Section
				title="Email (SMTP)"
				description="Used by email notification channels. Recipients are configured per channel in the Notifications tab."
			>
				<div className="grid gap-3 sm:grid-cols-2">
					<Field label="SMTP host" error={errors.smtp_host?.message}>
						<Input
							{...form.register("smtp_host")}
							placeholder="smtp.example.com"
						/>
					</Field>
					<Field label="SMTP port" error={errors.smtp_port?.message}>
						<Input
							type="number"
							{...form.register("smtp_port", { valueAsNumber: true })}
						/>
					</Field>
					<Field label="Username" error={errors.smtp_user?.message}>
						<Input {...form.register("smtp_user")} autoComplete="off" />
					</Field>
					<Field label="Password" error={errors.smtp_pass?.message}>
						<Input
							type="password"
							{...form.register("smtp_pass")}
							autoComplete="new-password"
						/>
					</Field>
				</div>
				<Field label="From address" error={errors.from?.message}>
					<Input
						{...form.register("from")}
						placeholder="subs-check <noreply@example.com>"
					/>
				</Field>
			</Section>

			<div className="flex justify-end">
				<Button type="submit" variant="success" loading={updateMut.isPending}>
					Save settings
				</Button>
			</div>
		</form>
	);
}
