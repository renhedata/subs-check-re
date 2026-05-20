import Fastify from 'fastify';
import { registerRoutes } from './server';

const app = Fastify({
	logger: true,
});

const PORT = parseInt(process.env.PORT || '3000', 10);

async function start() {
	await registerRoutes(app);

	try {
		await app.listen({ port: PORT, host: '0.0.0.0' });
		console.log(`Playwright service listening on port ${PORT}`);
	} catch (err) {
		app.log.error(err);
		process.exit(1);
	}
}

start();
