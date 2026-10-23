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
import { Card, Input, TextArea } from '@douyinfe/semi-ui';
import {
  groupTaskMultipliers,
  parseTaskExprPricing,
  parseTiersFromExpr,
} from '../../../../helpers';

// 参数值 | 倍率 | 备注
const GRID_STYLE = {
  display: 'grid',
  gridTemplateColumns: 'minmax(0, 1.2fr) 90px minmax(0, 2fr)',
  gap: 8,
  alignItems: 'center',
};

/**
 * 表达式计费的模型广场展示设置：表达式里识别出的参数倍率可逐条填写备注，
 * 另有一段计费说明。识别不了的表达式在模型广场只展示计费说明（不展示表达式）。
 * 仅影响展示，不参与计费。
 */
export default function ExprDisplaySettings({
  expr,
  notes,
  billingNote,
  onNotesChange,
  onBillingNoteChange,
  t,
}) {
  const currentNotes = notes || {};
  const taskPricing = parseTaskExprPricing(expr);
  const recognized =
    taskPricing !== null || parseTiersFromExpr(expr || '').length > 0;
  const groups = taskPricing
    ? groupTaskMultipliers(taskPricing.multipliers)
    : [];

  const setNote = (name, value, note) => {
    const next = { ...currentNotes, [name]: { ...(currentNotes[name] || {}) } };
    if (note) next[name][value] = note;
    else delete next[name][value];
    if (Object.keys(next[name]).length === 0) delete next[name];
    onNotesChange(next);
  };

  return (
    <Card
      bodyStyle={{ padding: 16 }}
      style={{ marginBottom: 16, background: 'var(--semi-color-fill-0)' }}
    >
      <div className='font-medium mb-1'>{t('模型广场展示')}</div>
      <div className='text-xs text-gray-500 mb-3'>
        {recognized
          ? t(
              '模型广场会以易读的方式展示该定价。下方备注仅用于展示，不影响计费。',
            )
          : t(
              '该表达式无法以易读方式展示。模型广场会隐藏表达式，只展示下方的计费说明。',
            )}
      </div>

      {groups.map((group) => (
        <Card
          key={group.name}
          bodyStyle={{ padding: 12 }}
          style={{ marginBottom: 12, background: 'var(--semi-color-bg-2)' }}
        >
          <div style={{ ...GRID_STYLE, marginBottom: 8 }}>
            <span className='font-mono font-semibold'>{group.name}</span>
            <span className='text-xs font-semibold text-gray-500'>
              {t('倍率')}
            </span>
            <span className='text-xs font-semibold text-gray-500'>
              {t('备注')}
            </span>
          </div>
          {group.rows.map((row) => (
            <div key={row.value} style={{ ...GRID_STYLE, marginBottom: 8 }}>
              <span className='font-mono'>{row.value}</span>
              <span className='font-mono'>×{row.ratio}</span>
              <Input
                value={currentNotes[group.name]?.[row.value] ?? ''}
                placeholder={t('备注（会显示在模型广场）')}
                maxLength={200}
                onChange={(value) => setNote(group.name, row.value, value)}
              />
            </div>
          ))}
        </Card>
      ))}

      <div className='text-sm font-medium mb-1'>{t('计费说明')}</div>
      <TextArea
        value={billingNote || ''}
        onChange={onBillingNoteChange}
        maxLength={1000}
        rows={3}
        placeholder={t(
          '说明该模型如何计费，例如：每秒 ¥0.18，按输出时长加参考视频时长计费。',
        )}
      />
    </Card>
  );
}
