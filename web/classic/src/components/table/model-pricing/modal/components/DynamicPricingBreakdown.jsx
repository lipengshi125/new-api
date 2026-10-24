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

import React from 'react';
import { Avatar, Tag, Table, Typography } from '@douyinfe/semi-ui';
import { IconPriceTag } from '@douyinfe/semi-icons';
import {
  parseTiersFromExpr,
  getCurrencyConfig,
  parseTaskExprPricing,
  groupTaskMultipliers,
} from '../../../../../helpers';
import { BILLING_PRICING_VARS } from '../../../../../constants';
import {
  splitBillingExprAndRequestRules,
  tryParseRequestRuleExpr,
  SOURCE_TIME,
  MATCH_RANGE,
  MATCH_EQ,
  MATCH_GTE,
  MATCH_LT,
  MATCH_CONTAINS,
  MATCH_EXISTS,
} from '../../../../../pages/Setting/Ratio/components/requestRuleExpr';

const { Text } = Typography;

const VAR_LABELS = { p: '输入', c: '输出' };
const OP_LABELS = { '<': '<', '<=': '≤', '>': '>', '>=': '≥' };
const TIME_FUNC_LABELS = { hour: '小时', minute: '分钟', weekday: '星期', month: '月份', day: '日期' };

function formatTokenHint(value) {
  const n = Number(value);
  if (!Number.isFinite(n) || n === 0) return '';
  if (n >= 1000000) return `${(n / 1000000).toFixed(n % 1000000 === 0 ? 0 : 1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(n % 1000 === 0 ? 0 : 1)}K`;
  return String(n);
}

function formatConditionSummary(conditions, t) {
  return conditions
    .map((c) => {
      if (c.var && c.op) {
        const varLabel = t(VAR_LABELS[c.var] || c.var);
        const hint = formatTokenHint(c.value);
        return `${varLabel} ${OP_LABELS[c.op] || c.op} ${hint || c.value}`;
      }
      return '';
    })
    .filter(Boolean)
    .join(' && ');
}


function describeCondition(cond, t) {
  if (cond.source === SOURCE_TIME) {
    const fn = t(TIME_FUNC_LABELS[cond.timeFunc] || cond.timeFunc);
    const tz = cond.timezone || 'UTC';
    if (cond.mode === MATCH_RANGE) {
      return `${fn} ${cond.rangeStart}:00~${cond.rangeEnd}:00 (${tz})`;
    }
    const opMap = { [MATCH_EQ]: '=', [MATCH_GTE]: '≥', [MATCH_LT]: '<' };
    return `${fn} ${opMap[cond.mode] || '='} ${cond.value} (${tz})`;
  }
  const src = cond.source === 'header' ? t('请求头') : t('请求参数');
  const path = cond.path || '';
  if (cond.mode === MATCH_EXISTS) return `${src} ${path} ${t('存在')}`;
  if (cond.mode === MATCH_CONTAINS) return `${src} ${path} ${t('包含')} "${cond.value}"`;
  const opMap = { eq: '=', gt: '>', gte: '≥', lt: '<', lte: '≤' };
  return `${src} ${path} ${opMap[cond.mode] || '='} ${cond.value}`;
}

function describeGroup(group, t) {
  const parts = (group.conditions || []).map((c) => describeCondition(c, t));
  return parts.join(' && ');
}

// 基础价格数字使用金黄色突出显示
const BASE_PRICE_COLOR = 'var(--semi-color-warning)';

function SectionHeader({ title, subtitle }) {
  return (
    <div className='flex items-center mb-4'>
      <Avatar size='small' color='amber' className='mr-2 shadow-md'>
        <IconPriceTag size={16} />
      </Avatar>
      <div>
        <Text className='text-lg font-medium'>{title}</Text>
        {subtitle && <div className='text-xs text-gray-600'>{subtitle}</div>}
      </div>
    </div>
  );
}

function BillingNote({ note, t }) {
  return (
    <div
      className='rounded-lg'
      style={{ padding: '10px 12px', background: 'var(--semi-color-fill-0)' }}
    >
      <div className='text-xs text-gray-500 mb-1'>{t('计费说明')}</div>
      <div className='text-sm' style={{ whiteSpace: 'pre-wrap' }}>
        {note}
      </div>
    </div>
  );
}

// 任务模型（按次/按秒）表达式：计费方式、基础价格、计费秒数、参数倍率、公式和示例
function TaskExprDetails({ pricing, modelData, t }) {
  const { symbol, rate } = getCurrencyConfig();
  const fmt = (usd) => `${symbol}${Number((usd * rate).toFixed(4))}`;
  const isSecond = pricing.unit === 'second';
  const unitLabel = isSecond ? t('秒') : t('次');
  const hasVideo = pricing.secondsSources.some((s) => s.kind === 'video');
  const groups = groupTaskMultipliers(pricing.multipliers);
  const notes = modelData?.custom_ratio_notes || {};

  const secondsText = pricing.secondsSources
    .map((s) =>
      s.kind === 'video'
        ? s.maxSeconds
          ? `${t('参考视频时长')}（${s.path}，${t('最多')} ${s.maxSeconds} ${t('秒')}）`
          : `${t('参考视频时长')}（${s.path}）`
        : `${t('输出时长')}（${s.path}）`,
    )
    .join(' + ');

  const formula = [
    fmt(pricing.unitPrice),
    ...(isSecond
      ? [
          pricing.secondsSources.length > 1
            ? `(${pricing.secondsSources
                .map((s) => (s.kind === 'video' ? t('参考视频秒数') : t('输出秒数')))
                .join(' + ')})`
            : t('输出秒数'),
        ]
      : []),
    ...groups.map((g) => `${g.name} ${t('倍率')}`),
    t('分组倍率'),
  ].join(' × ');

  const exampleOutput = 20;
  const exampleVideo = hasVideo ? 10 : 0;
  const exampleSeconds = isSecond ? exampleOutput + exampleVideo : 1;
  const choices = groups.map((g) => {
    const top = [...g.rows].sort((a, b) => b.ratio - a.ratio)[0];
    return { name: g.name, value: top.value, ratio: top.ratio };
  });
  const exampleMultiplier = choices.reduce((acc, c) => acc * c.ratio, 1);
  const exampleConditions = [
    ...(isSecond
      ? [
          hasVideo
            ? `${t('参考视频')} ${exampleVideo} ${t('秒')}，${t('输出')} ${exampleOutput} ${t('秒')}`
            : `${t('输出')} ${exampleOutput} ${t('秒')}`,
        ]
      : []),
    ...choices.map((c) => `${c.name}=${c.value}`),
  ];

  const rowStyle = {
    display: 'grid',
    gridTemplateColumns: '7rem minmax(0,1fr)',
    gap: 12,
    padding: '4px 0',
  };

  return (
    <div>
      <SectionHeader title={t('定价')} />
      <div
        className='rounded-lg mb-4'
        style={{ padding: '6px 12px', background: 'var(--semi-color-fill-0)' }}
      >
        <div style={rowStyle}>
          <span className='text-gray-500'>{t('计费方式')}</span>
          <span>{isSecond ? t('按秒计费') : t('按次计费')}</span>
        </div>
        <div style={rowStyle}>
          <span className='text-gray-500'>{t('基础价格')}</span>
          <span>
            <Text strong style={{ color: BASE_PRICE_COLOR }}>
              {fmt(pricing.unitPrice)}
            </Text>
            <span className='text-gray-500'> / {unitLabel}</span>
          </span>
        </div>
        {isSecond && (
          <div style={rowStyle}>
            <span className='text-gray-500'>{t('计费秒数')}</span>
            <span>{secondsText}</span>
          </div>
        )}
      </div>

      {groups.length > 0 && (
        <div className='mb-4'>
          <Text strong className='text-sm' style={{ display: 'block', marginBottom: 8 }}>
            {t('参数倍率')}
          </Text>
          <div
            className='overflow-hidden rounded-lg'
            style={{ border: '1px solid var(--semi-color-border)' }}
          >
            <table className='w-full text-sm' style={{ tableLayout: 'fixed' }}>
              <colgroup>
                <col style={{ width: '30%' }} />
                <col style={{ width: '18%' }} />
                <col />
              </colgroup>
              {groups.map((group) => (
                <tbody key={group.name}>
                  <tr style={{ background: 'var(--semi-color-fill-0)' }}>
                    <th
                      scope='rowgroup'
                      className='text-left font-mono font-semibold break-all'
                      style={{ padding: '8px 12px' }}
                    >
                      {group.name}
                    </th>
                    <th className='text-left text-xs text-gray-500' style={{ padding: '8px 12px' }}>
                      {t('倍率')}
                    </th>
                    <th className='text-left text-xs text-gray-500' style={{ padding: '8px 12px' }}>
                      {t('备注')}
                    </th>
                  </tr>
                  {group.rows.map((row) => (
                    <tr
                      key={row.value}
                      style={{ borderTop: '1px solid var(--semi-color-border)' }}
                    >
                      <td className='font-mono text-gray-600 break-all' style={{ padding: '8px 12px' }}>
                        {row.value}
                      </td>
                      <td className='font-mono font-semibold text-purple-600' style={{ padding: '8px 12px' }}>
                        ×{Number(row.ratio.toFixed(4))}
                      </td>
                      <td
                        className='text-gray-600'
                        style={{ padding: '8px 12px', wordBreak: 'break-word' }}
                      >
                        {notes[group.name]?.[row.value] || '-'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              ))}
            </table>
          </div>
          <div className='text-xs text-gray-500 mt-1'>
            {t('未列出的参数值按 ×1 计费。')}
          </div>
        </div>
      )}

      <div
        className='rounded-lg mb-4 text-sm'
        style={{ padding: '10px 12px', background: 'var(--semi-color-fill-0)' }}
      >
        <div>
          <span className='text-gray-500'>{t('计费公式')}：</span>
          <span className='font-mono'>{formula}</span>
        </div>
        <div style={{ marginTop: 4 }}>
          <span className='text-gray-500'>{t('示例')}：</span>
          {exampleConditions.join('，')}
          {exampleConditions.length > 0 && ' → '}
          <Text strong>{fmt(pricing.unitPrice * exampleSeconds * exampleMultiplier)}</Text>
          <span className='text-xs text-gray-500'> （{t('未含分组倍率')}）</span>
        </div>
      </div>

      {modelData?.billing_note && <BillingNote note={modelData.billing_note} t={t} />}
    </div>
  );
}

export default function DynamicPricingBreakdown({ billingExpr, modelData, t }) {
  const { symbol, rate } = getCurrencyConfig();

  const taskPricing = parseTaskExprPricing(billingExpr);
  if (taskPricing) {
    return <TaskExprDetails pricing={taskPricing} modelData={modelData} t={t} />;
  }

  const { billingExpr: baseExpr, requestRuleExpr: ruleExpr } =
    splitBillingExprAndRequestRules(billingExpr || '');

  const tiers = parseTiersFromExpr(baseExpr);
  const ruleGroups = tryParseRequestRuleExpr(ruleExpr || '');

  const hasTiers = tiers && tiers.length > 0;
  const hasRules = ruleGroups && ruleGroups.length > 0;

  // 识别不了的表达式：只展示计费说明，不暴露原始表达式
  if (!hasTiers) {
    return (
      <div>
        <SectionHeader title={t('动态计费')} />
        {modelData?.billing_note ? (
          <BillingNote note={modelData.billing_note} t={t} />
        ) : (
          <div className='text-sm text-gray-500'>
            {t('该模型采用动态计费，详情请联系管理员。')}
          </div>
        )}
      </div>
    );
  }

  const priceFields = BILLING_PRICING_VARS.map((v) => [v.field, v.shortLabel]);

  const tierColumns = [
    {
      title: t('档位'),
      dataIndex: 'label',
      render: (text, record) => (
        <div>
          <Tag color='blue' size='small'>{text || t('默认')}</Tag>
          {record.condSummary && (
            <div className='text-xs text-gray-500 mt-1'>{record.condSummary}</div>
          )}
        </div>
      ),
    },
    ...priceFields
      .filter(([field]) => hasTiers && tiers.some((tier) => tier[field] > 0))
      .map(([field, label]) => ({
        title: `${t(label)} (${symbol}/1M tokens)`,
        dataIndex: field,
        render: (v) => v > 0 ? <Text strong>{`${symbol}${(v * rate).toFixed(4)}`}</Text> : '-',
      })),
  ];

  const tierData = hasTiers
    ? tiers.map((tier, i) => ({
        key: `tier-${i}`,
        label: tier.label,
        condSummary: formatConditionSummary(tier.conditions, t),
        ...Object.fromEntries(priceFields.map(([field]) => [field, tier[field] || 0])),
      }))
    : [];

  // 只有一个档位时，档位名（如 base）对用户没有意义：直接列出基础价格
  const isSingleTier = tiers.length === 1;
  const singleTierPrices = isSingleTier
    ? priceFields.filter(([field]) => tiers[0][field] > 0)
    : [];

  return (
    <div>
      <SectionHeader
        title={isSingleTier ? t('定价') : t('动态计费')}
        subtitle={
          isSingleTier
            ? hasRules
              ? t('价格根据请求条件调整')
              : ''
            : t('价格根据用量档位和请求条件动态调整')
        }
      />

      {isSingleTier && (
        <div
          className='rounded-lg mb-4'
          style={{ padding: '6px 12px', background: 'var(--semi-color-fill-0)' }}
        >
          {singleTierPrices.map(([field, label]) => (
            <div
              key={field}
              style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}
            >
              <span className='text-gray-500'>{t(label)}</span>
              <span>
                <Text strong style={{ color: BASE_PRICE_COLOR }}>
                  {`${symbol}${Number((tiers[0][field] * rate).toFixed(4))}`}
                </Text>
                <span className='text-xs text-gray-500 font-normal'> / 1M tokens</span>
              </span>
            </div>
          ))}
        </div>
      )}

      {!isSingleTier && (
        <div style={{ marginBottom: 16 }}>
          <Text strong className='text-sm' style={{ display: 'block', marginBottom: 8 }}>
            {t('分档价格表')}
          </Text>
          <Table
            dataSource={tierData}
            columns={tierColumns}
            pagination={false}
            size='small'
            bordered={false}
            className='!rounded-lg'
          />
        </div>
      )}

      {hasRules && (
        <div style={{ marginBottom: 16 }}>
          <Text strong className='text-sm' style={{ display: 'block', marginBottom: 8 }}>
            {t('条件乘数')}
          </Text>
          {ruleGroups.map((group, gi) => (
            <div
              key={`group-${gi}`}
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                padding: '8px 12px',
                borderRadius: 6,
                background: 'var(--semi-color-fill-0)',
                marginBottom: 4,
              }}
            >
              <Text size='small'>{describeGroup(group, t)}</Text>
              <Tag color='orange' size='small'>{group.multiplier}x</Tag>
            </div>
          ))}
        </div>
      )}

    </div>
  );
}
