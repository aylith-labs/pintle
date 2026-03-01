import { existsSync } from 'node:fs';

export const BASE_DOMAIN = process.env.BASE_DOMAIN ?? 'lvh.me';
export const DASHBOARD_HOST = `proxy.${BASE_DOMAIN}`;
export const HOST_ADDRESS = existsSync('/.dockerenv') ? 'host.docker.internal' : 'localhost';
