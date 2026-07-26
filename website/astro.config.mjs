import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://alfian57.github.io',
  base: '/gin-kit',
  integrations: [
    starlight({
      title: 'gin-kit',
      description: 'An opinionated Go framework built on Gin.',
      logo: {
        src: './src/assets/gin-kit-mark.svg',
        alt: 'gin-kit',
      },
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/Alfian57/gin-kit',
        },
      ],
      customCss: ['./src/styles/custom.css'],
      editLink: {
        baseUrl: 'https://github.com/Alfian57/gin-kit/edit/main/website/',
      },
      sidebar: [
        {
          label: 'Start',
          items: [
            { label: 'Introduction', slug: 'index' },
            { label: 'Quickstart', slug: 'getting-started' },
            { label: 'Framework or starter?', slug: 'framework-vs-starter' },
          ],
        },
        {
          label: 'Core concepts',
          items: [
            { label: 'Architecture', slug: 'architecture' },
            { label: 'Responses and validation', slug: 'responses-validation' },
            { label: 'Filtering and pagination', slug: 'querying' },
            { label: 'Middleware and errors', slug: 'error-handling' },
            { label: 'Configuration', slug: 'configuration' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Database and ORM', slug: 'database-orm' },
            { label: 'Authentication and security', slug: 'auth-security' },
            { label: 'Caching and events', slug: 'caching-events' },
            { label: 'Observability', slug: 'observability' },
            { label: 'CLI and generators', slug: 'cli-generators' },
            { label: 'Customizing the runtime', slug: 'customization' },
            { label: 'UI mode', slug: 'ui-mode' },
            { label: 'Deployment', slug: 'deployment' },
          ],
        },
        {
          label: 'Project',
          items: [
            { label: 'AI agents', slug: 'ai-agents' },
            { label: 'Contributing', slug: 'contributing' },
            { label: 'Manifest v1 migration', slug: 'migration-v1' },
            { label: 'Releasing', slug: 'releasing' },
          ],
        },
      ],
    }),
  ],
});
