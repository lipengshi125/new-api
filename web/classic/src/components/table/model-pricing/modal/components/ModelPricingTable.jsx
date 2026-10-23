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
import { Avatar, Typography, Table, Tag } from '@douyinfe/semi-ui';
import { IconCoinMoneyStroked } from '@douyinfe/semi-icons';
import { calculateModelPrice, getModelPriceItems } from '../../../../../helpers';

const { Text } = Typography;

const ModelPricingTable = ({
  modelData,
  groupRatio,
  currency,
  siteDisplayType,
  tokenUnit,
  displayPrice,
  showRatio,
  usableGroup,
  autoGroups = [],
  t,
}) => {
  const modelEnableGroups = Array.isArray(modelData?.enable_groups)
    ? modelData.enable_groups
    : [];
  const autoChain = autoGroups.filter((g) => modelEnableGroups.includes(g));
  const renderGroupPriceTable = () => {
    // 仅展示模型可用的分组：模型 enable_groups 与用户可用分组的交集

    const availableGroups = Object.keys(usableGroup || {})
      .filter((g) => g !== '')
      .filter((g) => g !== 'auto')
      .filter((g) => modelEnableGroups.includes(g));

    // 准备表格数据
    const tableData = availableGroups.map((group) => {
      const priceData = modelData
        ? calculateModelPrice({
            record: modelData,
            selectedGroup: group,
            groupRatio,
            tokenUnit,
            displayPrice,
            currency,
            quotaDisplayType: siteDisplayType,
          })
        : { inputPrice: '-', outputPrice: '-', price: '-' };

      // 获取分组倍率
      const groupRatioValue =
        groupRatio && groupRatio[group] ? groupRatio[group] : 1;

      // 获取价格项（自定义参数倍率在下方单独的「参数倍率」表中展示）
      const priceItems = getModelPriceItems(priceData, t, siteDisplayType);

      return {
        key: group,
        group: group,
        ratio: groupRatioValue,
        billingType:
          modelData?.billing_mode === 'tiered_expr'
            ? t('动态计费')
            : modelData?.quota_type === 0
              ? t('按量计费')
              : modelData?.quota_type === 1
                ? t('按次计费')
                : '-',
        priceItems: priceItems,
      };
    });

    // 定义表格列
    const columns = [
      {
        title: t('分组'),
        dataIndex: 'group',
        render: (text) => (
          <Tag color='white' size='small' shape='circle'>
            {text}
            {t('分组')}
          </Tag>
        ),
      },
    ];

    const isDynamic = modelData?.billing_mode === 'tiered_expr';

    // 动态计费时始终显示倍率列，否则根据设置
    if (showRatio || isDynamic) {
      columns.push({
        title: t('分组倍率'),
        dataIndex: 'ratio',
        render: (text) => (
          <Tag color='blue' size='small' shape='circle'>
            {text}x
          </Tag>
        ),
      });
    }

    columns.push({
      title: t('计费类型'),
      dataIndex: 'billingType',
      render: (text) => {
        let color = 'white';
        if (text === t('按量计费')) color = 'violet';
        else if (text === t('按次计费')) color = 'teal';
        else if (text === t('动态计费')) color = 'amber';
        return (
          <Tag color={color} size='small' shape='circle'>
            {text || '-'}
          </Tag>
        );
      },
    });

    columns.push({
      title: siteDisplayType === 'TOKENS' ? t('计费摘要') : t('价格摘要'),
      dataIndex: 'priceItems',
      render: (items) => {
        if (items.length === 1 && items[0].isDynamic) {
          return (
            <Text type='tertiary' size='small'>
              {t('见上方动态计费详情')}
            </Text>
          );
        }
        return (
          <div className='space-y-1'>
            {items.map((item) => (
              <div key={item.key}>
                <div className={`font-semibold ${item.isCustomRatio ? 'text-purple-600' : 'text-orange-600'}`}>
                  {item.label} {item.value}
                </div>
                <div className='text-xs text-gray-500'>{item.suffix}</div>
              </div>
            ))}
          </div>
        );
      },
    });

    return (
      <Table
        dataSource={tableData}
        columns={columns}
        pagination={false}
        size='small'
        bordered={false}
        className='!rounded-lg'
      />
    );
  };

  // 参数倍率：按参数名分组，组头「参数名 | 倍率 | 备注」，其下每个参数值一行
  const renderCustomRatiosTable = () => {
    const customRatios = modelData?.custom_ratios || {};
    const notes = modelData?.custom_ratio_notes || {};
    const groups = Object.entries(customRatios)
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([paramName, valueMap]) => ({
        paramName,
        rows: Object.entries(valueMap || {}).sort(([a], [b]) =>
          a.localeCompare(b, undefined, { numeric: true }),
        ),
      }))
      .filter((group) => group.rows.length > 0);
    if (groups.length === 0) return null;

    const cellStyle = { padding: '8px 12px', verticalAlign: 'top' };
    return (
      <div className='mt-6'>
        <Text className='text-base font-medium'>{t('参数倍率')}</Text>
        <div
          className='mt-2 overflow-hidden rounded-lg'
          style={{ border: '1px solid var(--semi-color-border)' }}
        >
          <table className='w-full text-sm' style={{ tableLayout: 'fixed' }}>
            <colgroup>
              <col style={{ width: '30%' }} />
              <col style={{ width: '18%' }} />
              <col />
            </colgroup>
            {groups.map((group) => (
              <tbody key={group.paramName}>
                <tr style={{ background: 'var(--semi-color-fill-0)' }}>
                  <th
                    scope='rowgroup'
                    className='text-left font-mono font-semibold break-all'
                    style={cellStyle}
                  >
                    {group.paramName}
                  </th>
                  <th
                    className='text-left text-xs text-gray-500'
                    style={cellStyle}
                  >
                    {t('倍率')}
                  </th>
                  <th
                    className='text-left text-xs text-gray-500'
                    style={cellStyle}
                  >
                    {t('备注')}
                  </th>
                </tr>
                {group.rows.map(([paramValue, ratio]) => (
                  <tr
                    key={paramValue}
                    style={{ borderTop: '1px solid var(--semi-color-border)' }}
                  >
                    <td
                      className='font-mono text-gray-600 break-all'
                      style={cellStyle}
                    >
                      {paramValue}
                    </td>
                    <td
                      className='font-mono font-semibold text-purple-600'
                      style={cellStyle}
                    >
                      ×{ratio}
                    </td>
                    <td
                      className='text-gray-600'
                      style={{ ...cellStyle, wordBreak: 'break-word' }}
                    >
                      {notes[group.paramName]?.[paramValue] || '-'}
                    </td>
                  </tr>
                ))}
              </tbody>
            ))}
          </table>
        </div>
      </div>
    );
  };

  return (
    <div>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='orange' className='mr-2 shadow-md'>
          <IconCoinMoneyStroked size={16} />
        </Avatar>
        <div>
          <Text className='text-lg font-medium'>{t('分组价格')}</Text>
          <div className='text-xs text-gray-600'>
            {t('不同用户分组的价格信息')}
          </div>
        </div>
      </div>
      {autoChain.length > 0 && (
        <div className='flex flex-wrap items-center gap-1 mb-4'>
          <span className='text-sm text-gray-600'>{t('auto分组调用链路')}</span>
          <span className='text-sm'>→</span>
          {autoChain.map((g, idx) => (
            <React.Fragment key={g}>
              <Tag color='white' size='small' shape='circle'>
                {g}
                {t('分组')}
              </Tag>
              {idx < autoChain.length - 1 && <span className='text-sm'>→</span>}
            </React.Fragment>
          ))}
        </div>
      )}
      {renderGroupPriceTable()}
      {renderCustomRatiosTable()}
    </div>
  );
};

export default ModelPricingTable;
