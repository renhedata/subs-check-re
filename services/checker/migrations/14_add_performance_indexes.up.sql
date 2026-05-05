CREATE INDEX IF NOT EXISTS idx_check_results_job_alive ON check_results (job_id, alive);
CREATE INDEX IF NOT EXISTS idx_check_jobs_sub_status ON check_jobs (subscription_id, status);
CREATE INDEX IF NOT EXISTS idx_check_jobs_user_status ON check_jobs (user_id, status);
