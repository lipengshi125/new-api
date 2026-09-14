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
import { Button, Card, Input, Space } from '@douyinfe/semi-ui';
import { IconDelete, IconPlus } from '@douyinfe/semi-icons';

export default function CustomRatiosEditor({ customRatios, onChange, t }) {
  const handleAddParam = () => {
    const newCustomRatios = { ...customRatios };
    const paramName = `param_${Object.keys(newCustomRatios).length + 1}`;
    newCustomRatios[paramName] = { '': 1.0 };
    onChange(newCustomRatios);
  };

  const handleDeleteParam = (paramName) => {
    const newCustomRatios = { ...customRatios };
    delete newCustomRatios[paramName];
    onChange(newCustomRatios);
  };

  const handleParamNameChange = (oldName, newName) => {
    if (oldName === newName) return;
    const newCustomRatios = {};
    for (const key in customRatios) {
      if (key === oldName) {
        newCustomRatios[newName] = customRatios[key];
      } else {
        newCustomRatios[key] = customRatios[key];
      }
    }
    onChange(newCustomRatios);
  };

  const handleAddValue = (paramName) => {
    const newCustomRatios = { ...customRatios };
    newCustomRatios[paramName] = { ...newCustomRatios[paramName], '': 1.0 };
    onChange(newCustomRatios);
  };

  const handleDeleteValue = (paramName, valueName) => {
    const newCustomRatios = { ...customRatios };
    const newValues = { ...newCustomRatios[paramName] };
    delete newValues[valueName];
    newCustomRatios[paramName] = newValues;
    onChange(newCustomRatios);
  };

  const handleValueNameChange = (paramName, oldValueName, newValueName) => {
    if (oldValueName === newValueName) return;
    const newCustomRatios = { ...customRatios };
    const newValues = {};
    for (const key in newCustomRatios[paramName]) {
      if (key === oldValueName) {
        newValues[newValueName] = newCustomRatios[paramName][key];
      } else {
        newValues[key] = newCustomRatios[paramName][key];
      }
    }
    newCustomRatios[paramName] = newValues;
    onChange(newCustomRatios);
  };

  const handleRatioChange = (paramName, valueName, ratio) => {
    const numRatio = parseFloat(ratio);
    if (isNaN(numRatio)) return;
    const newCustomRatios = { ...customRatios };
    newCustomRatios[paramName] = {
      ...newCustomRatios[paramName],
      [valueName]: numRatio,
    };
    onChange(newCustomRatios);
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

      {Object.keys(customRatios || {}).length === 0 ? (
        <div className='text-center text-gray-400 py-4'>
          {t('暂无自定义参数，点击"添加参数"创建')}
        </div>
      ) : null}

      {Object.entries(customRatios || {}).map(([paramName, valueMap]) => (
        <Card
          key={paramName}
          bodyStyle={{ padding: 12 }}
          style={{ marginBottom: 12, background: 'var(--semi-color-bg-2)' }}
        >
          <div className='flex items-center gap-2 mb-2'>
            <Input
              value={paramName}
              placeholder={t('参数名 (如 resolution)')}
              onChange={(value) => handleParamNameChange(paramName, value)}
              style={{ flex: 1 }}
            />
            <Button
              size='small'
              type='danger'
              icon={<IconDelete />}
              onClick={() => handleDeleteParam(paramName)}
            />
          </div>

          {Object.entries(valueMap || {}).map(([valueName, ratio]) => (
            <Space key={valueName} style={{ width: '100%', marginBottom: 8 }}>
              <Input
                value={valueName}
                placeholder={t('参数值 (如 4k)')}
                onChange={(value) =>
                  handleValueNameChange(paramName, valueName, value)
                }
                style={{ width: 120 }}
              />
              <span>=</span>
              <Input
                type='number'
                value={ratio}
                placeholder='1.0'
                onChange={(value) =>
                  handleRatioChange(paramName, valueName, value)
                }
                suffix='x'
                style={{ width: 100 }}
                step={0.1}
                min={0}
              />
              <Button
                size='small'
                type='tertiary'
                icon={<IconDelete />}
                onClick={() => handleDeleteValue(paramName, valueName)}
              />
            </Space>
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
