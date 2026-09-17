/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import { useMemo } from 'react';

// 管理员在「顶栏管理」中配置的自定义导航项：完整的 http(s) 地址在新标签页打开，
// 其余一律视为站内路径。其他协议（javascript:、data: 等）直接丢弃，
// 避免存储的配置把导航项变成脚本执行入口。
const sanitizeCustomNavUrl = (raw) => {
  if (typeof raw !== 'string') return '';
  const trimmed = raw.trim();
  if (trimmed === '') return '';
  if (/^https?:\/\//i.test(trimmed)) return trimmed;
  if (/^[a-z][a-z0-9+.-]*:/i.test(trimmed) || trimmed.startsWith('//')) {
    return '';
  }
  return `/${trimmed.replace(/^\/+/, '')}`;
};

const buildCustomNavLinks = (custom) => {
  if (!Array.isArray(custom)) return [];

  return custom.reduce((links, entry, index) => {
    if (!entry || typeof entry !== 'object') return links;
    if (entry.enabled === false) return links;

    const name = typeof entry.name === 'string' ? entry.name.trim() : '';
    const url = sanitizeCustomNavUrl(entry.url);
    if (name === '' || url === '') return links;

    const isExternal = /^https?:\/\//i.test(url);
    links.push({
      text: name,
      itemKey: `custom-${index}`,
      isCustom: true,
      requireAuth: entry.requireAuth === true,
      ...(isExternal ? { isExternal: true, externalLink: url } : { to: url }),
    });
    return links;
  }, []);
};

export const useNavigation = (t, docsLink, headerNavModules) => {
  const mainNavLinks = useMemo(() => {
    // 默认配置，如果没有传入配置则显示所有模块
    const defaultModules = {
      home: true,
      console: true,
      pricing: true,
      docs: true,
      about: true,
    };

    // 使用传入的配置或默认配置
    const modules = headerNavModules || defaultModules;

    const allLinks = [
      {
        text: t('首页'),
        itemKey: 'home',
        to: '/',
      },
      {
        text: t('控制台'),
        itemKey: 'console',
        to: '/console',
      },
      {
        text: t('模型广场'),
        itemKey: 'pricing',
        to: '/pricing',
      },
      ...(docsLink
        ? [
            {
              text: t('文档'),
              itemKey: 'docs',
              isExternal: true,
              externalLink: docsLink,
            },
          ]
        : []),
      {
        text: t('关于'),
        itemKey: 'about',
        to: '/about',
      },
    ];

    // 根据配置过滤导航链接
    const enabledLinks = allLinks.filter((link) => {
      if (link.itemKey === 'docs') {
        return docsLink && modules.docs;
      }
      if (link.itemKey === 'pricing') {
        // 支持新的pricing配置格式
        return typeof modules.pricing === 'object'
          ? modules.pricing.enabled
          : modules.pricing;
      }
      return modules[link.itemKey] === true;
    });

    return [...enabledLinks, ...buildCustomNavLinks(modules.custom)];
  }, [t, docsLink, headerNavModules]);

  return {
    mainNavLinks,
  };
};
