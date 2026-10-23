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
import { Button, Card, Input } from '@douyinfe/semi-ui';
import { IconDelete, IconPlus } from '@douyinfe/semi-icons';

// 参数名 | 倍率 | 备注 三列，备注列最宽
const GRID_STYLE = {
  display: 'grid',
  gridTemplateColumns: 'minmax(0, 1.2fr) 110px minmax(0, 2fr) 32px',
  gap: 8,
  alignItems: 'center',
};

export default function CustomRatiosEditor({
  customRatios,
  customRatioNotes,
  onChange,
  t,
}) {
  const ratios = customRatios || {};
  const notes = customRatioNotes || {};

  const emit = (nextRatios, nextNotes) => onChange(nextRatios, nextNotes);

  const handleAddParam = () => {
    const paramName = `param_${Object.keys(ratios).length + 1}`;
    emit({ ...ratios, [paramName]: { '': 1.0 } }, notes);
  };

  const handleDeleteParam = (paramName) => {
    const nextRatios = { ...ratios };
    const nextNotes = { ...notes };
    delete nextRatios[paramName];
    delete nextNotes[paramName];
    emit(nextRatios, nextNotes);
  };

  // 重命名时保持参数顺序，并把备注一并迁移
  const renameKey = (source, oldKey, newKey) => {
    const result = {};
    for (const key in source) {
      result[key === oldKey ? newKey : key] = source[key];
    }
    return result;
  };

  const handleParamNameChange = (oldName, newName) => {
    if (oldName === newName) return;
    emit(
      renameKey(ratios, oldName, newName),
      renameKey(notes, oldName, newName),
    );
  };

  const handleAddValue = (paramName) => {
    emit({ ...ratios, [paramName]: { ...ratios[paramName], '': 1.0 } }, notes);
  };

  const handleDeleteValue = (paramName, valueName) => {
    const nextValues = { ...ratios[paramName] };
    delete nextValues[valueName];
    const nextNoteValues = { ...(notes[paramName] || {}) };
    delete nextNoteValues[valueName];
    emit(
      { ...ratios, [paramName]: nextValues },
      { ...notes, [paramName]: nextNoteValues },
    );
  };

  const handleValueNameChange = (paramName, oldValueName, newValueName) => {
    if (oldValueName === newValueName) return;
    emit(
      {
        ...ratios,
        [paramName]: renameKey(ratios[paramName], oldValueName, newValueName),
      },
      {
        ...notes,
        [paramName]: renameKey(
          notes[paramName] || {},
          oldValueName,
          newValueName,
        ),
      },
    );
  };

  const handleRatioChange = (paramName, valueName, ratio) => {
    const numRatio = parseFloat(ratio);
    if (isNaN(numRatio)) return;
    emit(
      {
        ...ratios,
        [paramName]: { ...ratios[paramName], [valueName]: numRatio },
      },
      notes,
    );
  };

  const handleNoteChange = (paramName, valueName, note) => {
    emit(ratios, {
      ...notes,
      [paramName]: { ...(notes[paramName] || {}), [valueName]: note },
    });
  };

  return (
    <Card
      bodyStyle={{ padding: 16 }}
      style={{ marginBottom: 16, background: 'var(--semi-color-fill-0)' }}
    >
      <div className='flex items-center justify-between mb-3'>
        <div className='font-medium'>{t('自定义倍率参数')}</div>
        <Button size='small' icon={<IconPlus />} onClick={handleAddParam}>
          {t('添加参数')}
        </Button>
      </div>
      <div className='text-xs text-gray-500 mb-3'>
        {t('从请求中提取自定义参数并应用对应倍率，如 resolution=4k → x1.5')}
      </div>

      {Object.keys(ratios).length === 0 ? (
        <div className='text-center text-gray-400 py-4'>
          {t('暂无自定义参数，点击"添加参数"创建')}
        </div>
      ) : null}

      {Object.entries(ratios).map(([paramName, valueMap]) => (
        <Card
          key={paramName}
          bodyStyle={{ padding: 12 }}
          style={{ marginBottom: 12, background: 'var(--semi-color-bg-2)' }}
        >
          {/* 组头：参数名 | 倍率 | 备注 */}
          <div style={{ ...GRID_STYLE, marginBottom: 8 }}>
            <Input
              value={paramName}
              placeholder={t('参数名 (如 resolution)')}
              onChange={(value) => handleParamNameChange(paramName, value)}
            />
            <div className='text-xs font-semibold text-gray-500'>
              {t('倍率')}
            </div>
            <div className='text-xs font-semibold text-gray-500'>
              {t('备注')}
            </div>
            <Button
              size='small'
              type='danger'
              icon={<IconDelete />}
              onClick={() => handleDeleteParam(paramName)}
            />
          </div>

          {/* 参数值行：参数值 | 倍率 | 备注 */}
          {Object.entries(valueMap || {}).map(([valueName, ratio]) => (
            <div key={valueName} style={{ ...GRID_STYLE, marginBottom: 8 }}>
              <Input
                value={valueName}
                placeholder={t('参数值 (如 4k)')}
                onChange={(value) =>
                  handleValueNameChange(paramName, valueName, value)
                }
              />
              <Input
                type='number'
                value={ratio}
                placeholder='1.0'
                onChange={(value) =>
                  handleRatioChange(paramName, valueName, value)
                }
                suffix='x'
                step={0.1}
                min={0}
              />
              <Input
                value={notes[paramName]?.[valueName] ?? ''}
                placeholder={t('备注（会显示在模型广场）')}
                maxLength={200}
                onChange={(value) =>
                  handleNoteChange(paramName, valueName, value)
                }
              />
              <Button
                size='small'
                type='tertiary'
                icon={<IconDelete />}
                onClick={() => handleDeleteValue(paramName, valueName)}
              />
            </div>
          ))}

          <Button
            size='small'
            type='tertiary'
            icon={<IconPlus />}
            onClick={() => handleAddValue(paramName)}
            style={{ marginTop: 8 }}
          >
            {t('添加值')}
          </Button>
        </Card>
      ))}
    </Card>
  );
}
